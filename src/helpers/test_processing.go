package handlers

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"C"
	"github.com/DataDog/go-python3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/jung-kurt/gofpdf"
	"github.com/neo4j/neo4j-go-driver/neo4j"
	"gopkg.in/gographics/imagick.v2/imagick"

	"qbot_webserver/src/repositories"
)

const (
	GradingErrorNotification   = "Test requires correction!"
	TestGradedNotification     = "Test has been graded!"
	TestCorrectionNotification = "Test has been corrected!"

	margin           = 25
	topMargin        = 20
	headerCellWidth  = 65
	headerCellHeight = 7
	headerLineHeight = 7
	tableCellSize    = 8
	spacingLarge     = 25
	spacingSmall     = 12
	a4height         = 297
	a4width          = 210

	testTemplatesFolder = "test_templates"
)

func GenerateTestTemplate(test repositories.Test, s3Bucket string, s3Region string, s3Profile string, logger *log.Logger) (string, error) {
	filenamePrefix := strings.ReplaceAll(fmt.Sprintf("/tmp/%s_%s", test.Subject, test.Name), " ", "_")
	filenamePDF := fmt.Sprintf("%s.pdf", filenamePrefix)
	filenameJPG := fmt.Sprintf("%s.jpg", filenamePrefix)

	err := createLocalPDF(test, filenamePDF)
	if err != nil {
		return "", err
	}

	err = convertPdfToJPG(filenamePDF, filenameJPG)
	if err != nil {
		return "", err
	}
	defer deleteFromLocal(filenamePDF, filenameJPG, logger)

	return uploadToS3(s3Bucket, s3Region, s3Profile, filenameJPG, testTemplatesFolder)
}

func GradeTestImage(logger *log.Logger, session neo4j.Session, teacherID int, test repositories.CompletedTest,
	s3Bucket string, s3Region string, s3Profile string,
) {
	test = runPythonScriptToGrade(test, s3Bucket, s3Region, s3Profile)
	fmt.Printf("%+v\n", test)

	//query := fmt.Sprintf(`
	//	MATCH (s:Student {ID:$studentID})-[st:COMPLETED]->(t:Test {testID:$testID})-[tp:ADDED_BY]->(p:Teacher {ID:$teacherID})
	//	SET st.grade = $grade, st.timestamp = $timestamp, st.gradedTestImage = $gradedTestImage,
	//			st.testImage = $testImage, st.notificationMessage = '%s'
	//`, TestGradedNotification)
	//params := map[string]interface{}{
	//	"studentID":       studentID,
	//	"testID":          test.ID,
	//	"teacherID":       teacherID,
	//	"grade":           grade,
	//	"timestamp":       time.Now().Unix(),
	//	"gradedTestImage": gradedImageURL,
	//	"testImage":       testImageURL,
	//}
	//
	//err := helpers.WriteTX(session, query, params)
	//if err != nil {
	//	logger.Printf("error grading test %d: %s", test.ID, err.Error())
	//}
}

func runPythonScriptToGrade(test repositories.CompletedTest, s3Bucket string, s3Region string, s3Profile string) repositories.CompletedTest {
	defer python3.Py_Finalize()
	python3.Py_Initialize()
	python3.PyRun_SimpleString(getGradingScript(
		test, s3Bucket, s3Region, s3Profile,
	))

	evalModule := python3.PyImport_AddModule("__main__")
	evalDict := python3.PyModule_GetDict(evalModule)

	email := python3.PyDict_GetItemString(evalDict, "student_email")
	if email == nil {
		python3.PyErr_Print()
		fmt.Println("grading error: could not retrieve email")
	} else {
		retString := python3.PyUnicode_AsUTF8(email)
		test.Author.Email = retString
	}
	email.DecRef()

	link := python3.PyDict_GetItemString(evalDict, "graded_image_link")
	if link == nil {
		python3.PyErr_Print()
		fmt.Println("grading error: could not retrieve link")
	} else {
		retString := python3.PyUnicode_AsUTF8(link)
		test.GradedTestImageURL = retString
	}
	link.DecRef()

	grade := python3.PyDict_GetItemString(evalDict, "grade")
	if grade == nil {
		python3.PyErr_Print()
		fmt.Println("grading error: could not retrieve grade")
	} else {
		retInt := python3.PyLong_AsLong(grade)
		test.Grade = retInt
	}
	grade.DecRef()

	answers := python3.PyDict_GetItemString(evalDict, "answers")
	if answers == nil {
		python3.PyErr_Print()
		fmt.Println("grading error: could not retrieve answers")
	} else {
		retString := python3.PyUnicode_AsUTF8(answers)
		fmt.Printf("%+v\n", retString)

		answerMap, err := GetAnswerMapFromPythonString(retString)
		if err != nil {
			fmt.Printf("grading error: could not convert answers: %s\n", err.Error())
		}

		test.Answers = answerMap
	}
	answers.DecRef()

	return test
}

func convertPdfToJPG(filenamePDF string, filenameJPG string) error {
	imagick.Initialize()
	defer imagick.Terminate()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	if err := mw.SetResolution(300, 300); err != nil {
		return err
	}
	if err := mw.ReadImage(filenamePDF); err != nil {
		return err
	}
	if err := mw.SetImageAlphaChannel(imagick.ALPHA_CHANNEL_FLATTEN); err != nil {
		return err
	}
	if err := mw.SetCompressionQuality(95); err != nil {
		return err
	}
	mw.SetIteratorIndex(0)
	if err := mw.SetFormat("jpg"); err != nil {
		return err
	}

	return mw.WriteImage(filenameJPG)
}

func createLocalPDF(test repositories.Test, filename string) error {
	header := make([]string, test.NrAnswerOptions+1)
	gridSizes := make([]uint, test.NrAnswerOptions+1)
	header[0] = "Nr."
	gridSizes[0] = 1

	for i := 1; i <= test.NrAnswerOptions; i++ {
		header[i] = string(rune('A' + i - 1))
		gridSizes[i] = 1
	}

	contents := make([][]string, test.NrQuestions)
	for i := 0; i < test.NrQuestions; i++ {
		row := []string{strconv.Itoa(i + 1)}
		for j := 0; j < test.NrAnswerOptions; j++ {
			row = append(row, "")
		}

		contents[i] = row
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, topMargin, margin)
	pdf.AddPage()
	pdf.SetFont("Times", "B", 14)
	pdf.Cell(headerCellWidth, headerCellHeight, "First Name:")
	pdf.Cell(headerCellWidth, headerCellHeight, "_________________________________")
	pdf.Ln(headerLineHeight)
	pdf.Cell(headerCellWidth, headerCellHeight, "Last Name:")
	pdf.Cell(headerCellWidth, headerCellHeight, "_________________________________")
	pdf.Ln(headerLineHeight)
	pdf.Cell(headerCellWidth, headerCellHeight, "University e-mail address:")
	pdf.Cell(headerCellWidth, headerCellHeight, "_________________________________")
	pdf.Ln(headerLineHeight)
	pdf.Cell(headerCellWidth, headerCellHeight, "Year:")
	pdf.Cell(headerCellWidth, headerCellHeight, "_________________________________")
	pdf.Ln(headerLineHeight)
	pdf.Cell(headerCellWidth, headerCellHeight, "Group:")
	pdf.Cell(headerCellWidth, headerCellHeight, "_________________________________")
	pdf.Ln(headerLineHeight)
	pdf.Cell(headerCellWidth, headerCellHeight, "Specialization:")
	pdf.Cell(headerCellWidth, headerCellHeight, "_________________________________")

	pdf.Ln(spacingLarge)
	pdf.SetFont("Times", "B", 20)
	pdf.CellFormat(0, 10, test.Name, "", 0, "C", false, 0, "")
	pdf.Ln(spacingSmall)
	pdf.SetFont("Times", "B", 17)
	pdf.CellFormat(0, 10, test.Subject, "", 0, "C", false, 0, "")
	pdf.Ln(spacingLarge)

	pdf.SetFont("Times", "B", 14)
	pdf.SetFillColor(255, 255, 255)
	tableWidth := float64(0)
	for index, str := range header {
		width := float64(tableCellSize)
		if index == 0 {
			width = tableCellSize * 2
		}
		pdf.CellFormat(width, tableCellSize, str, "1", 0, "C", true, 0, "")
		tableWidth += width
	}
	spaceBetweenTables := a4width - 2*margin - 2*tableWidth
	pdf.Cell(spaceBetweenTables, tableCellSize, "")
	for index, str := range header {
		width := float64(tableCellSize)
		if index == 0 {
			width = tableCellSize * 2
		}
		pdf.CellFormat(width, tableCellSize, str, "1", 0, "C", true, 0, "")
	}

	pdf.Ln(-1)

	nrRows := int(math.Ceil(float64(test.NrQuestions) / float64(2)))

	for questionNr := 0; questionNr < nrRows; questionNr++ {
		for index, str := range contents[questionNr] {
			width := float64(tableCellSize)
			if index == 0 {
				width = tableCellSize * 2
			}
			pdf.CellFormat(width, tableCellSize, str, "1", 0, "C", true, 0, "")
		}
		pdf.Cell(spaceBetweenTables, tableCellSize, "")
		if nrRows+questionNr < len(contents) {
			for index, str := range contents[nrRows+questionNr] {
				width := float64(tableCellSize)
				if index == 0 {
					width = tableCellSize * 2
				}
				pdf.CellFormat(width, tableCellSize, str, "1", 0, "C", true, 0, "")
			}
		}

		pdf.Ln(-1)
	}

	return pdf.OutputFileAndClose(filename)
}

func uploadToS3(s3Bucket string, s3Region string, s3Profile string, filename string, folder string) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(s3Region),
		Credentials: credentials.NewSharedCredentials("", s3Profile),
	}))

	uploader := s3manager.NewUploader(sess)

	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open file %q, %v", filename, err)
	}

	split := strings.Split(filename, "/")
	filename = split[len(split)-1]

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", folder, filename)),
		Body:   f,
		ACL:    aws.String(s3.ObjectCannedACLPublicRead),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file, %v", err)
	}

	return aws.StringValue(&result.Location), nil
}

func deleteFromLocal(filenamePDF string, filenameJPG string, logger *log.Logger) {
	err := os.Remove(filenamePDF)
	if err != nil {
		logger.Printf("could not delete %s: %s", filenamePDF, err.Error())
	}
	err = os.Remove(filenameJPG)
	if err != nil {
		logger.Printf("could not delete %s: %s", filenameJPG, err.Error())
	}
}

func getGradingScript(test repositories.CompletedTest, s3Bucket string, s3Region string, s3Profile string) string {
	return fmt.Sprintf(`

import math
import os
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


def find_rotated_perspective_answers(image_url, template_url, correct_answers, nr_questions, nr_answers,
									 multiple_answers, partial_scoring, total_points, ex_officio, s3_bucket, s3_region,
									 s3_profile):
	# current_image = cv.imread("test.png")
	current_image = imutils.url_to_image(image_url)
	current_image = cv.blur(current_image, (3, 3))
	current_image = cv.cvtColor(current_image, cv.COLOR_BGR2RGB)

	# template = cv.imread("template.jpg")
	template = imutils.url_to_image(template_url)
	template = cv.blur(template, (3, 3))
	template = cv.cvtColor(template, cv.COLOR_BGR2RGB)

	orb = cv.ORB_create(nfeatures=1000000)
	bf = cv.BFMatcher(cv.NORM_HAMMING, crossCheck=True)
	normalize_kernel = cv.getStructuringElement(cv.MORPH_ELLIPSE, (100, 100))

	left_image, right_image, student_email_area = get_tables(current_image, template, orb, bf, normalize_kernel)

	answers_left, table_left = find_table(left_image, 10, 5, math.ceil(nr_questions / 2), nr_answers, multiple_answers)
	answers_right, table_right = find_table(right_image, 10, 5, math.floor(nr_questions / 2), nr_answers,
											multiple_answers)
	all_answers = answers_left + answers_right

	student_email = get_student_email(student_email_area)
	graded_image_link = send_image_to_s3(image_url, table_left, table_right, s3_bucket, s3_region, s3_profile)
	grade, percentage = calculate_grade(all_answers, correct_answers, multiple_answers, partial_scoring, total_points,
										ex_officio)

	return student_email, '%%s' %% all_answers, grade, percentage, graded_image_link


student_email, answers, grade, percentage, graded_image_link = find_rotated_perspective_answers(
	"%s", "%s", %v, %d, %d, %s, %s, %d, %d, "%s", "%s", "%s"
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
