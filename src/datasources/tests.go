package datasources

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func AddTestAnswers(session neo4j.Session, token string, testID int, answers map[int][]string) error {
	// TODO

	return nil
}

func AddFeedbackForTest(session neo4j.Session, token string, testID int, feedback string) error {
	// TODO

	return nil
}

func OverwriteGradeForTest(session neo4j.Session, token string, testID int, studentID int, newGrade int) error {
	// TODO

	return nil
}

func SignalErrorForTest(session neo4j.Session, token string, testID int) error {
	// TODO

	return nil
}

func GetNotificationTests(session neo4j.Session, token string) ([]repositories.CompletedTest, error) {
	// TODO

	return []repositories.CompletedTest{}, nil
}

func GradeTest(session neo4j.Session, token string, test repositories.CompletedTest) error {
	// TODO

	return nil
}

func AddTest(session neo4j.Session, token string, test repositories.Test) error {
	// TODO

	return nil
}

func DeleteTest(session neo4j.Session, token string, testID int) error {
	// TODO

	return nil
}

func GetTests(session neo4j.Session, path string, token string, testID int, searchString string) ([]repositories.CompletedTest, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return []repositories.CompletedTest{}, helpers.InvalidTokenError(path, err)
	}

	if tokenInfo.Label == teacherLabel {
		if testID != helpers.EmptyIntParameter {
			return getAllCompletedTestsForTeacher(session, testID)
		} else {
			return getAllTestsForTeacher(session, tokenInfo.ID, searchString)
		}
	} else {
		return getAllCompletedTestsForStudent(session, tokenInfo.ID, searchString)
	}

	return []repositories.CompletedTest{}, nil
}

func getAllCompletedTestsForStudent(session neo4j.Session, studentID int, searchString string) ([]repositories.CompletedTest, error) {
	extraCondition := ""
	if searchString != helpers.EmptyStringParameter {
		value := 8
		extraCondition = fmt.Sprintf(`
			AND (apoc.text.distance(t.name, '%s') < %d OR apoc.text.distance(subj.name, '%s') < %d) 
		`, searchString, value, searchString, value)
	}

	query := fmt.Sprintf(`
		MATCH (g:Group)<-[sg:MEMBER_OF]-(s:Student)-[st:COMPLETED]->(t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE s.ID = $studentID %s 
		RETURN s.ID, s.email, s.firstName, s.lastName, g.gID, 
				t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, count(st) as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName, 
				st.testImage, st.gradedTestImage, st.grade, st.timestamp, st.correctedGrade, st.correctedGradeTimestamp, st.notificationMessage, st.feedback
	`, extraCondition)

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest
		records, err := tx.Run(query, map[string]interface{}{"studentID": studentID})
		if err != nil {
			return []repositories.CompletedTest{}, err
		}

		for records.Next() {
			record := records.Record()
			completedTest, err := getCompletedTestFromTestQuery(record)
			if err != nil {
				return []repositories.CompletedTest{}, err
			}

			results = append(results, completedTest)
		}

		return results, nil
	})

	if err != nil {
		return []repositories.CompletedTest{}, err
	}

	return testResults.([]repositories.CompletedTest), nil
}

func getAllTestsForTeacher(session neo4j.Session, teacherID int, searchString string) ([]repositories.CompletedTest, error) {
	extraCondition := ""
	if searchString != helpers.EmptyStringParameter {
		value := 8
		extraCondition = fmt.Sprintf(`
			AND (apoc.text.distance(t.name, '%s') < %d OR apoc.text.distance(subj.name, '%s') < %d) 
		`, searchString, value, searchString, value)
	}

	query := fmt.Sprintf(`
		MATCH (Student)-[st:COMPLETED]->(t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher)  
		WHERE p.ID = $teacherID %s
		RETURN t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, count(st) as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName
	`, extraCondition)

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest
		records, err := tx.Run(query, map[string]interface{}{"teacherID": teacherID})
		if err != nil {
			return []repositories.CompletedTest{}, err
		}

		for records.Next() {
			record := records.Record()
			test, err := getTestFromTestQuery(record)
			if err != nil {
				return []repositories.CompletedTest{}, err
			}

			results = append(results, repositories.CompletedTest{Test: test})
		}

		return results, nil
	})

	if err != nil {
		return []repositories.CompletedTest{}, err
	}

	return testResults.([]repositories.CompletedTest), nil
}

func getAllCompletedTestsForTeacher(session neo4j.Session, testID int) ([]repositories.CompletedTest, error) {
	query := `
		MATCH (g:Group)<-[sg:MEMBER_OF]-(s:Student)-[st:COMPLETED]->(t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE t.testID = $testID 
		RETURN s.ID, s.email, s.firstName, s.lastName, g.gID, 
				t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, count(st) as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName, 
				st.testImage, st.gradedTestImage, st.grade, st.timestamp, st.correctedGrade, st.correctedGradeTimestamp, st.notificationMessage, st.feedback
	`

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest
		records, err := tx.Run(query, map[string]interface{}{"testID": testID})
		if err != nil {
			return []repositories.CompletedTest{}, err
		}

		for records.Next() {
			record := records.Record()
			completedTest, err := getCompletedTestFromTestQuery(record)
			if err != nil {
				return []repositories.CompletedTest{}, err
			}

			results = append(results, completedTest)
		}

		return results, nil
	})

	if err != nil {
		return []repositories.CompletedTest{}, err
	}

	return testResults.([]repositories.CompletedTest), nil
}

func getCompletedTestFromTestQuery(record neo4j.Record) (repositories.CompletedTest, error) {
	test, err := getTestFromTestQuery(record)
	if err != nil {
		return repositories.CompletedTest{}, err
	}

	student, err := getStudentFromTestQuery(record)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	completedTestURL, err := helpers.GetStringParameterFromQuery(record, "st.testImage", true, true)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	gradedTestURL, err := helpers.GetStringParameterFromQuery(record, "st.gradedTestImage", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	grade, err := helpers.GetIntParameterFromQuery(record, "st.grade", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	gradeTimestamp, err := helpers.GetIntParameterFromQuery(record, "st.timestamp", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	correctedGrade, err := helpers.GetIntParameterFromQuery(record, "st.correctedGrade", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	correctedGradeTimestamp, err := helpers.GetIntParameterFromQuery(record, "st.correctedGradeTimestamp", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	notification, err := helpers.GetStringParameterFromQuery(record, "st.notificationMessage", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	feedback, err := helpers.GetStringParameterFromQuery(record, "st.feedback", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}

	return repositories.CompletedTest{
		Test:                    test,
		TestImageURL:            completedTestURL,
		GradedTestImageURL:      gradedTestURL,
		Grade:                   grade,
		GradeTimestamp:          gradeTimestamp,
		CorrectedGrade:          correctedGrade,
		CorrectedGradeTimestamp: correctedGradeTimestamp,
		NotificationMessage:     notification,
		Feedback:                feedback,
		Author:                  student,
	}, nil
}

func getTestFromTestQuery(record neo4j.Record) (repositories.Test, error) {
	teacher, err := getTeacherFromTestQuery(record)
	if err != nil {
		return repositories.Test{}, err
	}
	testID, err := helpers.GetIntParameterFromQuery(record, "t.testID", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	testSubject, err := helpers.GetStringParameterFromQuery(record, "subj.name", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	testName, err := helpers.GetStringParameterFromQuery(record, "t.name", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	nrQuestions, err := helpers.GetIntParameterFromQuery(record, "t.nrQuestions", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	nrAnswers, err := helpers.GetIntParameterFromQuery(record, "t.nrAnswers", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	points, err := helpers.GetIntParameterFromQuery(record, "t.points", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	exOfficioPoints, err := helpers.GetIntParameterFromQuery(record, "t.exOfficio", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	multipleAnswersAllowed, err := helpers.GetBoolParameterFromQuery(record, "t.multipleAnswersAllowed", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	enablePartialScoring, err := helpers.GetBoolParameterFromQuery(record, "t.enablePartialScoring", true, false)
	if err != nil {
		return repositories.Test{}, err
	}
	mandatoryToPass, err := helpers.GetBoolParameterFromQuery(record, "t.mandatoryToPass", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	templateURL, err := helpers.GetStringParameterFromQuery(record, "t.template", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	gradeCount, err := helpers.GetIntParameterFromQuery(record, "nrTestsGraded", true, true)
	if err != nil {
		return repositories.Test{}, err
	}
	answers, err := helpers.GetAnswerMapFromQuery(record, "t.answers", true, true)
	if err != nil {
		return repositories.Test{}, err
	}

	return repositories.Test{
		ID:                     testID,
		Subject:                testSubject,
		Name:                   testName,
		NrQuestions:            nrQuestions,
		NrAnswerOptions:        nrAnswers,
		TotalPoints:            points,
		ExOfficioPoints:        exOfficioPoints,
		MultipleAnswersAllowed: multipleAnswersAllowed,
		EnablePartialScoring:   enablePartialScoring,
		MandatoryToPass:        mandatoryToPass,
		TemplateImageURL:       templateURL,
		NrTestsGraded:          gradeCount,
		Teacher:                teacher,
		CorrectAnswers:         answers,
	}, nil
}

func getStudentFromTestQuery(record neo4j.Record) (repositories.Student, error) {
	user, err := getUserFromTestQuery(record, "s")
	if err != nil {
		return repositories.Student{}, err
	}

	studentGroup, err := helpers.GetIntParameterFromQuery(record, "g.gID", true, true)
	if err != nil {
		return repositories.Student{}, err
	}

	return repositories.Student{
		User:  user,
		Group: studentGroup,
	}, nil
}

func getTeacherFromTestQuery(record neo4j.Record) (repositories.Professor, error) {
	user, err := getUserFromTestQuery(record, "p")
	if err != nil {
		return repositories.Professor{}, err
	}

	return repositories.Professor{
		User: user,
	}, nil
}

func getUserFromTestQuery(record neo4j.Record, nodeName string) (repositories.User, error) {
	ID, err := helpers.GetIntParameterFromQuery(record, fmt.Sprintf("%s.ID", nodeName), true, true)
	if err != nil {
		return repositories.User{}, err
	}
	email, err := helpers.GetStringParameterFromQuery(record, fmt.Sprintf("%s.email", nodeName), true, true)
	if err != nil {
		return repositories.User{}, err
	}
	firstName, err := helpers.GetStringParameterFromQuery(record, fmt.Sprintf("%s.firstName", nodeName), true, true)
	if err != nil {
		return repositories.User{}, err
	}
	lastName, err := helpers.GetStringParameterFromQuery(record, fmt.Sprintf("%s.firstName", nodeName), true, true)
	if err != nil {
		return repositories.User{}, err
	}

	return repositories.User{
		ID:        ID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}, nil
}
