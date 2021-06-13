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

func GetTests(session neo4j.Session, path string, token string, onlyGraded bool, testID int, searchString string, subject string) ([]repositories.CompletedTest, error) {
	// TODO

	// if a test has no completions, only return the test itself

	//search - check for similarity among title, subject
	//subject - get tests for this subject
	//only graded

	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return []repositories.CompletedTest{}, helpers.InvalidTokenError(path, err)
	}

	if tokenInfo.Label == teacherLabel {
		if testID != helpers.EmptyIntParameter {
			return getAllTestCompletionsQuery(session, testID)
		}
	} else {

	}

	return []repositories.CompletedTest{}, nil
}

func getAllTestCompletionsQuery(session neo4j.Session, testID int) ([]repositories.CompletedTest, error) {
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

			teacher, err := getTeacherFromTestQuery(record)
			if err != nil {
				continue
			}
			testSubject, err := helpers.GetStringParameterFromQuery(record, "subj.name", true, true)
			if err != nil {
				continue
			}
			testName, err := helpers.GetStringParameterFromQuery(record, "t.name", true, true)
			if err != nil {
				continue
			}
			nrQuestions, err := helpers.GetIntParameterFromQuery(record, "t.nrQuestions", true, true)
			if err != nil {
				continue
			}
			nrAnswers, err := helpers.GetIntParameterFromQuery(record, "t.nrAnswers", true, true)
			if err != nil {
				continue
			}
			points, err := helpers.GetIntParameterFromQuery(record, "t.points", true, true)
			if err != nil {
				continue
			}
			exOfficioPoints, err := helpers.GetIntParameterFromQuery(record, "t.exOfficio", true, true)
			if err != nil {
				continue
			}
			multipleAnswersAllowed, err := helpers.GetBoolParameterFromQuery(record, "t.multipleAnswersAllowed", true, true)
			if err != nil {
				continue
			}
			enablePartialScoring, err := helpers.GetBoolParameterFromQuery(record, "t.enablePartialScoring", true, false)
			if err != nil {
				continue
			}
			mandatoryToPass, err := helpers.GetBoolParameterFromQuery(record, "t.mandatoryToPass", true, true)
			if err != nil {
				continue
			}
			templateURL, err := helpers.GetStringParameterFromQuery(record, "t.template", true, true)
			if err != nil {
				continue
			}
			gradeCount, err := helpers.GetIntParameterFromQuery(record, "nrTestsGraded", true, true)
			if err != nil {
				continue
			}
			answers, err := helpers.GetAnswerMapFromQuery(record, "t.answers", true, true)
			if err != nil {
				continue
			}

			test := repositories.Test{
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
			}

			student, err := getStudentFromTestQuery(record)
			if err != nil {
				continue
			}
			completedTestURL, err := helpers.GetStringParameterFromQuery(record, "st.testImage", true, true)
			if err != nil {
				continue
			}
			gradedTestURL, err := helpers.GetStringParameterFromQuery(record, "st.gradedTestImage", true, false)
			if err != nil {
				continue
			}
			grade, err := helpers.GetIntParameterFromQuery(record, "st.grade", true, false)
			if err != nil {
				continue
			}
			gradeTimestamp, err := helpers.GetIntParameterFromQuery(record, "st.timestamp", true, false)
			if err != nil {
				continue
			}
			correctedGrade, err := helpers.GetIntParameterFromQuery(record, "st.correctedGrade", true, false)
			if err != nil {
				continue
			}
			correctedGradeTimestamp, err := helpers.GetIntParameterFromQuery(record, "st.correctedGradeTimestamp", true, false)
			if err != nil {
				continue
			}
			notification, err := helpers.GetStringParameterFromQuery(record, "st.notificationMessage", true, false)
			if err != nil {
				continue
			}
			feedback, err := helpers.GetStringParameterFromQuery(record, "st.feedback", true, false)
			if err != nil {
				continue
			}

			completedTest := repositories.CompletedTest{
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
