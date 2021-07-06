package handlers

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/DataDog/go-python3"
	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/repositories"
)

func GradeTestImage(logger *log.Logger, session neo4j.Session, teacherID int, test repositories.CompletedTest,
	s3Bucket string, s3Region string, s3Profile string,
) {
	var err error
	attempts := 3
	for attempts > 0 {
		test, err = runPythonScriptToGrade(test, s3Bucket, s3Region, s3Profile)
		if err != nil {
			logger.Printf(err.Error())
			attempts -= 1
		} else {
			attempts = 0
		}
	}
	if err != nil {
		return
	}

	answerString, err := GetStringFromAnswerMap(test.Answers)
	if err != nil {
		logger.Printf("grading error for test %d: could not get string from answer map: %s", test.ID, err.Error())
	}

	query := fmt.Sprintf(`
		MATCH (s:Student {email:'%s'}), (t:Test {testID:$testID})-[tp:ADDED_BY]->(p:Teacher {ID:$teacherID})
		MERGE (s)-[st:COMPLETED]->(t)
		SET st.grade = $grade, st.timestamp = $timestamp, st.gradedTestImage = $gradedTestImage,
				st.testImage = $testImage, st.notificationMessage = '%s', st.answers = '%s'
	`, test.Author.Email, TestGradedNotification, answerString)
	params := map[string]interface{}{
		"testID":          test.ID,
		"teacherID":       teacherID,
		"grade":           test.Grade,
		"timestamp":       time.Now().Unix(),
		"gradedTestImage": test.GradedTestImageURL,
		"testImage":       test.TestImageURL,
	}

	err = WriteTX(session, query, params)
	if err != nil {
		logger.Printf("grading error for test %d: transaction failed: %s", test.ID, err.Error())
	}
}

func runPythonScriptToGrade(test repositories.CompletedTest, s3Bucket string, s3Region string, s3Profile string) (repositories.CompletedTest, error) {
	if !python3.Py_IsInitialized() {
		python3.Py_Initialize()
	}
	fmt.Printf("%+v\n", test)
	python3.PyRun_SimpleString(getGradingScript(
		test, s3Bucket, s3Region, s3Profile,
	))

	evalModule := python3.PyImport_AddModule("__main__")
	evalDict := python3.PyModule_GetDict(evalModule)

	email := python3.PyDict_GetItemString(evalDict, "student_email")
	if email == nil {
		python3.PyErr_Print()
		return test, fmt.Errorf("grading error for test %d: could not retrieve email\n", test.ID)
	} else {
		retString := python3.PyUnicode_AsUTF8(email)
		test.Author.Email = retString
		email.DecRef()
	}

	link := python3.PyDict_GetItemString(evalDict, "graded_image_link")
	if link == nil {
		python3.PyErr_Print()
		return test, fmt.Errorf("grading error for test %d: could not retrieve link\n", test.ID)
	} else {
		retString := python3.PyUnicode_AsUTF8(link)
		test.GradedTestImageURL = retString
		link.DecRef()
	}

	grade := python3.PyDict_GetItemString(evalDict, "grade")
	if grade == nil {
		python3.PyErr_Print()
		return test, fmt.Errorf("grading error for test %d: could not retrieve grade\n", test.ID)
	} else {
		retInt := python3.PyLong_AsLong(grade)
		test.Grade = retInt
		grade.DecRef()
	}

	answers := python3.PyDict_GetItemString(evalDict, "answers")
	if answers == nil {
		python3.PyErr_Print()
		return test, fmt.Errorf("grading error for test %d: could not retrieve answers\n", test.ID)
	} else {
		retString := python3.PyUnicode_AsUTF8(answers)
		fmt.Printf("%+v\n", retString)
		answerMap, err := GetAnswerMapFromPythonString(retString)
		if err != nil {
			return test, fmt.Errorf("grading error for test %d: could not convert answers: %s\n", test.ID, err.Error())
		}

		test.Answers = answerMap
		answers.DecRef()
	}

	return test, nil
}

func getGradingScript(test repositories.CompletedTest, s3Bucket string, s3Region string, s3Profile string) string {
	return fmt.Sprintf(`

import json
import math
import os
import re
import string

import boto3
import cv2 as cv
import imutils
import numpy as np
from boto3.s3.transfer import S3Transfer
from pytesseract import pytesseract


# orientation = 0 -> horizontal = rows
# orientation = 1 -> vertical = columns

def find_lines(grayscale_image, nr_rows, nr_columns, orientation=0):
    image_height = grayscale_image.shape[1]
    threshold_same_line = image_height / 20

    if orientation == 1:
        grayscale_image = np.rot90(grayscale_image).copy()

    edges = cv.Sobel(grayscale_image, ddepth=cv.CV_64F, dx=0, dy=1)
    edges = np.abs(edges)
    edges = edges / edges.max()

    _, edges_threshold = cv.threshold(edges, 0.4, 255, cv.THRESH_BINARY_INV)

    mask = (edges_threshold == 0) * 1
    all_lines = np.sum(mask, axis=1)
    all_lines = all_lines.argsort()

    if orientation == 1:
        num_lines = 70
    else:
        num_lines = 100

    line_thickness = 2
    edges_threshold = np.dstack((edges_threshold, edges_threshold, edges_threshold))
    lines = []

    for i in range(1, num_lines + 1):
        cv.line(edges_threshold, (0, all_lines[-i]), (grayscale_image.shape[1], all_lines[-i]), (0, 0, 255),
                line_thickness)
        lines.append([(0, all_lines[-i]), (grayscale_image.shape[1], all_lines[-i])])

    lines.sort(key=lambda coords: coords[0][1])

    distict_lines = []
    distict_lines.append(lines[0])

    for line in lines:
        difference = line[0][1] - distict_lines[-1][0][1]
        if difference > threshold_same_line:
            distict_lines.append(line)

    if orientation == 0:
        correct_lines = distict_lines[-nr_rows - 1:]
    else:
        correct_lines = distict_lines[:nr_columns + 1]

        for line in correct_lines:
            val00, val01 = reversed(line[0])
            val10, val11 = reversed(line[1])
            val00 = image_height - val00 - line_thickness
            val10 = val00
            line[0] = (val00, val01)
            line[1] = (val10, val11)
        correct_lines.sort(key=lambda coords: coords[1][0])

    return correct_lines


def choice_nr_to_answer(choice):
    return chr(65 + choice)


def find_table(grayscale_image, threshold_mean_difference, threshold_min_difference_for_choice,
               nr_questions, nr_answers, multiple_answers=False):
    horizontal_lines = find_lines(grayscale_image.copy(), nr_questions, nr_answers, orientation=0)
    vertical_lines = find_lines(grayscale_image.copy(), nr_questions, nr_answers, orientation=1)
    line_thickness = 2

    mean_color = grayscale_image.mean(axis=0).mean(axis=0)

    color_image = np.dstack((grayscale_image, grayscale_image, grayscale_image))

    for line in horizontal_lines:
        cv.line(color_image, line[0], line[1], (255, 0, 0), line_thickness)
    for line in vertical_lines:
        cv.line(color_image, line[0], line[1], (0, 0, 255), line_thickness)

    padding = 0.25

    answers = []

    for i in range(len(horizontal_lines) - 1):

        answers_considered = []

        for j in range(len(vertical_lines) - 1):

            x_min = vertical_lines[j][0][0]
            x_max = vertical_lines[j + 1][1][0]
            x_window = (x_max - x_min) * padding
            x_min = int(x_min + x_window)
            x_max = int(x_max - x_window)

            y_min = horizontal_lines[i][0][1]
            y_max = horizontal_lines[i + 1][1][1]
            y_window = (y_max - y_min) * padding
            y_min = int(y_min + y_window)
            y_max = int(y_max - y_window)

            patch = grayscale_image[y_min:y_max, x_min:x_max]
            mean_patch = np.round(patch.mean())

            if mean_color - mean_patch > threshold_mean_difference:
                answers_considered.append((j, mean_patch, (x_min, y_min), (x_max, y_max)))

            cv.putText(color_image, str(mean_patch)[:3], (int(x_min + x_window / 2), int(y_min + (y_max - y_min) / 2)),
                       cv.FONT_HERSHEY_COMPLEX, 0.55, (0, 0, 0), 1)
            cv.rectangle(color_image, (x_min, y_min), (x_max, y_max), color=(211, 211, 211), thickness=line_thickness)

        if not multiple_answers:
            if len(answers_considered) == 1:
                choice = answers_considered[0][0]
                complete_choice = answers_considered[0]
            elif len(answers_considered) == 0:
                choice = -1
            else:
                min_mean_patch = answers_considered[0]
                max_mean_patch = answers_considered[0]

                for answer in answers_considered:
                    if answer[1] < min_mean_patch[1]:
                        min_mean_patch = answer
                    elif answer[1] > max_mean_patch[1]:
                        max_mean_patch = answer

                if max_mean_patch[1] - min_mean_patch[1] >= threshold_min_difference_for_choice:
                    choice = min_mean_patch[0]
                    complete_choice = min_mean_patch
                else:
                    choice = -1

            answers.append(choice_nr_to_answer(choice))

            if choice != -1:
                cv.rectangle(color_image, complete_choice[2], complete_choice[3], color=(0, 200, 0),
                             thickness=line_thickness)
        else:
            current_answers = []
            for complete_choice in answers_considered:
                cv.rectangle(color_image, complete_choice[2], complete_choice[3], color=(0, 200, 0),
                             thickness=line_thickness)

                current_answers.append(choice_nr_to_answer(complete_choice[0]))

            answers.append(current_answers)

    return answers, color_image


def get_image_name(image_name):
    return image_name.split('/')[-1]


def get_image_name_prefix(image_name):
    return get_image_name(image_name).split('.')[0]


def calculate_grade(all_answers, correct_answers, multiple_answers, partial_scoring, total_points, ex_officio):
    total_nr_ex = len(correct_answers)
    points_per_question = float((total_points - ex_officio) / total_nr_ex)
    points_received = float(ex_officio)

    if len(all_answers) < len(correct_answers):
        all_answers += [[]] * (len(correct_answers) - len(all_answers))
    elif len(all_answers) > len(correct_answers):
        all_answers = all_answers[:len(correct_answers)]

    for index, correct_answer in enumerate(correct_answers):
        if not multiple_answers and all_answers[index] == correct_answer:
            points_received += points_per_question

        elif multiple_answers:
            answer_set = set(all_answers[index])
            correct_answer_set = set(correct_answer)
            common = answer_set & correct_answer_set

            if partial_scoring:
                nr_wrong = answer_set - correct_answer_set
                divide_by = float(len(correct_answer_set) + len(nr_wrong))
                if divide_by != 0:
                    partial_scoring_coefficient = float(len(common)) / divide_by
                else:
                    partial_scoring_coefficient = 1
                points_received += points_per_question * partial_scoring_coefficient
            elif len(common) == len(correct_answer_set) and len(common) == len(answer_set):
                points_received += points_per_question

    return round(points_received), round(points_received * 100 / total_points)


def normalize(image, kernel, black_threshold=200, white_threshold=210):
    gray = cv.cvtColor(image, cv.COLOR_BGR2GRAY)

    dilated = cv.morphologyEx(gray, cv.MORPH_DILATE, kernel)
    median = cv.medianBlur(dilated, 15)
    median_difference = 255 - cv.subtract(median, gray)

    image = cv.normalize(median_difference, None, 0, 255, cv.NORM_MINMAX)
    image[image < black_threshold] = 0
    image[image > white_threshold] = 255

    return image


def get_image_areas(image, normalize_kernel):
    image = normalize(image, normalize_kernel)
    image_height, image_width = image.shape
    image_tables = image[int(image_height * 0.25):, :]

    left_image = image_tables[:, :image_width // 2]
    right_image = image_tables[:, image_width // 2:]

    header = image[:int(image_height * 0.25), int(image_width * 0.4):int(image_width * 0.85)]
    # cv.imwrite("header.png", header)

    return left_image, right_image, header


def get_tables(image, template, orb, matcher, normalize_kernel):
    key_point_template, descriptor_template = orb.detectAndCompute(template, None)
    key_point_image, descriptor_image = orb.detectAndCompute(image, None)

    matches = matcher.match(descriptor_image, descriptor_template)
    matches = sorted(matches, key=lambda x: x.distance)
    points_template = np.zeros((len(matches), 2))

    points_image = np.zeros((len(matches), 2))

    for i, match in enumerate(matches):
        points_template[i, :] = key_point_template[match.trainIdx].pt
        points_image[i, :] = key_point_image[match.queryIdx].pt

    homography, mask = cv.findHomography(points_image, points_template, cv.RANSAC)

    height, width, _ = template.shape
    aligned_image = cv.warpPerspective(image, homography, (width, height), flags=cv.INTER_NEAREST)

    return get_image_areas(aligned_image, normalize_kernel)


def send_image_to_s3(image_url, table_left, table_right, s3_bucket, s3_region, s3_profile):
    image_name = get_image_name(image_url)
    graded_image_name = get_image_name_prefix(str(image_name)) + "_graded.png"
    s3_graded_image_path = 'test_graded/' + graded_image_name

    h_img = cv.hconcat([table_left, table_right])
    cv.imwrite(graded_image_name, h_img)

    session = boto3.Session(profile_name=s3_profile)
    client = session.client('s3', s3_region)
    transfer = S3Transfer(client)
    transfer.upload_file(graded_image_name, s3_bucket, s3_graded_image_path, extra_args={'ACL': 'public-read'})

    os.remove(graded_image_name)

    return '%%s/%%s/%%s' %% (client.meta.endpoint_url, s3_bucket, s3_graded_image_path)


def detect_email(image, s3_profile):
    session = boto3.Session(profile_name=s3_profile)
    client = session.client('textract')

    is_success, im_buf_arr = cv.imencode(".jpg", image)
    byte_im = im_buf_arr.tobytes()

    response = client.detect_document_text(Document={
        'Bytes': byte_im,
    })

    email = json.dumps(response)
    email = re.findall(r'[\w\. -]+@[\w\. -]+', email)[0]
    email = email.strip().translate(str.maketrans('', '', string.whitespace)).lower().split("@")[0]

    return email + "@stud.ase.ro"


def get_student_email(student_email_area):
    # pipeline = keras_ocr.pipeline.Pipeline()
    # text = pipeline.recognize(student_email_area)

    # reader = easyocr.Reader(['en'], gpu=False)
    # text = reader.readtext(student_email_area, detail=0)

    email = ""
    text = pytesseract.image_to_string(student_email_area)
    for item in text.split("\n"):
        if "@" in item:
            email = item.strip().translate(str.maketrans('', '', string.whitespace)).lower().split("@")[0]

    return email + "@stud.ase.ro"


def edges_det(img, min_val, max_val):
    img = cv.cvtColor(img, cv.COLOR_BGR2GRAY)

    # Applying blur and threshold
    img = cv.bilateralFilter(img, 9, 75, 75)
    img = cv.adaptiveThreshold(img, 255, cv.ADAPTIVE_THRESH_GAUSSIAN_C, cv.THRESH_BINARY, 115, 4)

    # Median blur replace center pixel by median of pixels under kelner
    # => removes thin details
    img = cv.medianBlur(img, 11)

    # Add black border - detection of border touching pages
    # Contour can't touch side of image
    img = cv.copyMakeBorder(img, 5, 5, 5, 5, cv.BORDER_CONSTANT, value=[0, 0, 0])

    return cv.Canny(img, min_val, max_val)


def four_corners_sort(pts):
    diff = np.diff(pts, axis=1)
    summ = pts.sum(axis=1)
    return np.array([pts[np.argmin(summ)],
                     pts[np.argmax(diff)],
                     pts[np.argmax(summ)],
                     pts[np.argmin(diff)]])


def contour_offset(cnt, offset):
    cnt += offset
    cnt[cnt < 0] = 0
    return cnt


def find_page_contours(edges):
    # Getting contours  
    contours, hierarchy = cv.findContours(edges, cv.RETR_TREE, cv.CHAIN_APPROX_SIMPLE)

    # Finding biggest rectangle otherwise return original corners
    height = edges.shape[0]
    width = edges.shape[1]
    MIN_COUNTOUR_AREA = height * width * 0.5
    MAX_COUNTOUR_AREA = (width - 10) * (height - 10)

    max_area = MIN_COUNTOUR_AREA
    page_contour = np.array([[0, 0],
                             [0, height - 5],
                             [width - 5, height - 5],
                             [width - 5, 0]])

    for cnt in contours:
        perimeter = cv.arcLength(cnt, True)
        approx = cv.approxPolyDP(cnt, 0.03 * perimeter, True)

        # Page has 4 corners and it is convex
        if (len(approx) == 4 and
                cv.isContourConvex(approx) and
                max_area < cv.contourArea(approx) < MAX_COUNTOUR_AREA):
            max_area = cv.contourArea(approx)
            page_contour = approx[:, 0]

    # Sort corners and offset them
    page_contour = four_corners_sort(page_contour)

    return contour_offset(page_contour, (-5, -5))


def persp_transform(img, s_points):
    # Euclidean distance - calculate maximum height and width
    height = max(np.linalg.norm(s_points[0] - s_points[1]),
                 np.linalg.norm(s_points[2] - s_points[3]))
    width = max(np.linalg.norm(s_points[1] - s_points[2]),
                np.linalg.norm(s_points[3] - s_points[0]))

    # Create target points
    t_points = np.array([[0, 0],
                         [0, height],
                         [width, height],
                         [width, 0]], np.float32)

    # getPerspectiveTransform() needs float32
    if s_points.dtype != np.float32:
        s_points = s_points.astype(np.float32)

    M = cv.getPerspectiveTransform(s_points, t_points)

    img = cv.warpPerspective(img, M, (int(width), int(height)))
    image_height, image_width, _ = img.shape
    img = img[int(image_height * 0.015):int(image_height * 0.985), int(image_width * 0.015):int(image_width * 0.985)]

    return img


def find_rotated_perspective_answers(image_url, template_url, correct_answers, nr_questions, nr_answers,
                                     multiple_answers, partial_scoring, total_points, ex_officio, s3_bucket, s3_region,
                                     s3_profile, align=True):
    # current_image = cv.imread("test.png")
    current_image = imutils.url_to_image(image_url)
    current_image = cv.blur(current_image, (3, 3))
    current_image = cv.cvtColor(current_image, cv.COLOR_BGR2RGB)

    # template = cv.imread("template.jpg")
    template = imutils.url_to_image(template_url)
    template = cv.blur(template, (3, 3))
    template = cv.cvtColor(template, cv.COLOR_BGR2RGB)

    orb = cv.ORB_create(nfeatures=1000)
    bf = cv.BFMatcher(cv.NORM_HAMMING, crossCheck=True)
    normalize_kernel = cv.getStructuringElement(cv.MORPH_ELLIPSE, (50, 50))

    if align:
        left_image, right_image, student_email_area = get_tables(current_image, template, orb, bf, normalize_kernel)
    else:
        edges_image = edges_det(current_image, 200, 250)

        # Close gaps between edges (double page close => rectangle kernel)
        edges_image = cv.morphologyEx(edges_image, cv.MORPH_CLOSE, np.ones((5, 11)))

        page_contour = find_page_contours(edges_image)
        current_image = persp_transform(current_image, page_contour)

        left_image, right_image, student_email_area = get_image_areas(current_image, normalize_kernel)

    answers_left, table_left = find_table(left_image, 10, 5, math.ceil(nr_questions / 2), nr_answers, multiple_answers)
    answers_right, table_right = find_table(right_image, 10, 5, math.floor(nr_questions / 2), nr_answers,
                                            multiple_answers)
    all_answers = answers_left + answers_right

    # student_email = get_student_email(student_email_area)
    student_email = detect_email(student_email_area, s3_profile)
    graded_image_link = send_image_to_s3(image_url, table_left, table_right, s3_bucket, s3_region, s3_profile)
    grade, percentage = calculate_grade(all_answers, correct_answers, multiple_answers, partial_scoring, total_points,
                                        ex_officio)

    return student_email, '"%%s"' %% all_answers, grade, percentage, graded_image_link


student_email, answers, grade, percentage, graded_image_link = find_rotated_perspective_answers(
	"%s", "%s", %v, %d, %d, %s, %s, %d, %d, "%s", "%s", "%s", False
)

#print(student_email)
#print(answers)
#print(grade)
#print(percentage)
#print(graded_image_link)

	`, test.TestImageURL,
		test.Test.TemplateImageURL,
		getListOfListsFromMap(test.Test.CorrectAnswers),
		test.Test.NrQuestions,
		test.Test.NrAnswerOptions,
		getPythonBoolean(test.Test.MultipleAnswersAllowed),
		getPythonBoolean(test.Test.EnablePartialScoring),
		test.Test.TotalPoints,
		test.Test.ExOfficioPoints,
		s3Bucket,
		s3Region,
		s3Profile,
	)
}

func getPythonBoolean(b bool) string {
	if b {
		return "True"
	}

	return "False"
}

func getListOfListsFromMap(inputMap map[int][]string) string {
	keys := make([]int, len(inputMap))
	i := 0
	for k := range inputMap {
		keys[i] = k
		i++
	}
	sort.Ints(keys)

	listString := `[`
	for _, k := range keys {
		insideString := ``
		for _, value := range inputMap[k] {
			insideString += fmt.Sprintf("'%s',", value)
		}
		listString += fmt.Sprintf("[%s],", insideString)
	}
	listString += `]`

	return listString
}
