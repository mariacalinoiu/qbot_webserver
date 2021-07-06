package datasources

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

func AddTestAnswers(session neo4j.Session, path string, token string, testID int, answers map[int][]string) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.TeacherLabel {
		return helpers.InvalidTokenError(path, err)
	}

	answerString, err := helpers.GetStringFromAnswerMap(answers)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		MATCH (t:Test {testID:$testID})-[tp:ADDED_BY]->(p:Teacher {ID:$teacherID}) 
		SET t.answers = '%s'
	`, answerString)
	params := map[string]interface{}{
		"teacherID": tokenInfo.ID,
		"testID":    testID,
	}

	return helpers.WriteTX(session, query, params)
}

func AddFeedbackForTest(session neo4j.Session, path string, token string, testID int, feedback string) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.TeacherLabel {
		return helpers.InvalidTokenError(path, err)
	}

	query := fmt.Sprintf(`
		MATCH (t:Test {testID:$testID})-[tp:ADDED_BY]->(p:Teacher {ID:$teacherID}) 
		SET t.feedback = '%s'
	`, feedback)
	params := map[string]interface{}{
		"teacherID": tokenInfo.ID,
		"testID":    testID,
	}

	return helpers.WriteTX(session, query, params)
}

func OverwriteGradeForTest(session neo4j.Session, path string, token string, testID int, studentID int, newGrade int) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.TeacherLabel {
		return helpers.InvalidTokenError(path, err)
	}

	query := fmt.Sprintf(`
		MATCH (s:Student {ID:$studentID})-[st:COMPLETED]->(t:Test {testID:$testID})-[tp:ADDED_BY]->(p:Teacher {ID:$teacherID}) 
		SET st.correctedGrade = $newGrade, st.correctedGradeTimestamp = $newGradeTS, st.notificationMessage = '%s'
	`, helpers.TestCorrectionNotification)
	params := map[string]interface{}{
		"studentID":  studentID,
		"testID":     testID,
		"teacherID":  tokenInfo.ID,
		"newGrade":   newGrade,
		"newGradeTS": time.Now().Unix(),
	}

	return helpers.WriteTX(session, query, params)
}

func SignalErrorForTest(session neo4j.Session, path string, token string, testID int) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.StudentLabel {
		return helpers.InvalidTokenError(path, err)
	}

	query := fmt.Sprintf(`
		MATCH (s:Student)-[st:COMPLETED]->(t:Test) 
		WHERE s.ID = $studentID AND t.testID = $testID 
		SET st.notificationMessage = '%s'
	`, helpers.GradingErrorNotification)
	params := map[string]interface{}{
		"studentID": tokenInfo.ID,
		"testID":    testID,
	}

	return helpers.WriteTX(session, query, params)
}

func GradeTest(logger *log.Logger, session neo4j.Session, path string, token string, test repositories.CompletedTest, s3Bucket string, s3Region string, s3Profile string) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.TeacherLabel {
		return helpers.InvalidTokenError(path, err)
	}
	testDetails, err := GetTestDetails(session, path, token, test.Name, tokenInfo.ID)
	if err != nil {
		return err
	}
	testDetails.TestImageURL = test.TestImageURL

	go helpers.GradeTestImage(logger, session, tokenInfo.ID, testDetails, s3Bucket, s3Region, s3Profile)

	return nil
}

func GetTestDetails(session neo4j.Session, path string, token string, testName string, teacherID int) (repositories.CompletedTest, error) {
	query := fmt.Sprintf(`
		MATCH (t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE p.ID = $teacherID AND t.name = '%s' 
		RETURN t.testID 
	`, testName)

	testID, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var result int

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, map[string]interface{}{
			"teacherID": teacherID,
		})
		if err != nil {
			return 0, err
		}

		for records.Next() {
			record := records.Record()
			result, err = helpers.GetIntParameterFromQuery(record, "t.testID", true, true)
			if err != nil {
				return 0, err
			}

			return result, nil
		}

		return 0, nil
	})

	if err != nil || testID == 0 {
		return repositories.CompletedTest{}, fmt.Errorf("could not get test with given name: %s\n", testName)
	}

	testDetails, err := GetTests(session, path, token, testID.(int), helpers.EmptyStringParameter, true)
	if err != nil || len(testDetails) != 1 {
		return repositories.CompletedTest{}, fmt.Errorf("could not get test with given name: %s\n", testName)
	}

	return testDetails[0], nil
}

func AddTest(session neo4j.Session, path string, token string, test repositories.Test, s3Bucket string, s3Region string, s3Profile string, logger *log.Logger) (int, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.TeacherLabel {
		return 0, helpers.InvalidTokenError(path, err)
	}

	queryPrefix := ""
	testID := test.ID
	if test.ID != 0 {
		queryPrefix = `
			MATCH (t:Test {testID:$testID}) 
		`
	} else {
		testID, err = getNextNodeID(session, "Test", "testID")
		if err != nil {
			return 0, err
		}

		queryPrefix = `
			CREATE (t:Test {testID:$testID}) 
		`
	}

	templateURL, err := helpers.GenerateTestTemplate(test, s3Bucket, s3Region, s3Profile, logger)

	logger.Printf("generated template for test %d at %s\n", testID, templateURL)

	query := fmt.Sprintf(`
		%s 
		SET t.name='%s', t.nrQuestions=$nrQuestions, t.nrAnswers=$nrAnswers, t.points=$points, t.exOfficio=$exOfficio, 
			t.multipleAnswersAllowed=$multipleAnswersAllowed, t.enablePartialScoring=$enablePartialScoring, t.mandatoryToPass=$mandatoryToPass,
			t.template=$template 
	`, queryPrefix, test.Name)

	params := map[string]interface{}{
		"testID":                 testID,
		"nrQuestions":            test.NrQuestions,
		"nrAnswers":              test.NrAnswerOptions,
		"points":                 test.TotalPoints,
		"exOfficio":              test.ExOfficioPoints,
		"multipleAnswersAllowed": test.MultipleAnswersAllowed,
		"enablePartialScoring":   test.EnablePartialScoring,
		"mandatoryToPass":        test.MandatoryToPass,
		"template":               templateURL,
	}

	err = helpers.WriteTX(session, query, params)
	if err != nil {
		return 0, err
	}

	query = fmt.Sprintf(`
		MATCH (p:Teacher {ID:$teacherID}), (t:Test {testID:$testID}), (subj:Subject {name:'%s'}) 
		MERGE (t)-[r:ADDED_BY]->(p)
		MERGE (t)-[q:BELONGS_TO]->(subj)
	`, test.Subject)

	params = map[string]interface{}{
		"teacherID": tokenInfo.ID,
		"testID":    testID,
	}

	return testID, helpers.WriteTX(session, query, params)
}

func DeleteTest(session neo4j.Session, path string, token string, testID int) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil || tokenInfo.Label != repositories.TeacherLabel {
		return helpers.InvalidTokenError(path, err)
	}

	query := `
		MATCH (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE p.ID = $teacherID AND t.testID = $testID 
		DETACH DELETE t
	`
	params := map[string]interface{}{
		"teacherID": tokenInfo.ID,
		"testID":    testID,
	}

	return helpers.WriteTX(session, query, params)
}

func GetNotificationTests(session neo4j.Session, path string, token string) ([]repositories.CompletedTest, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return []repositories.CompletedTest{}, helpers.InvalidTokenError(path, err)
	}

	notificationMessages := make([]string, 2)
	if tokenInfo.Label == repositories.TeacherLabel {
		notificationMessages[0] = helpers.GradingErrorNotification
		notificationMessages[1] = helpers.TestGradedNotification
	} else {
		notificationMessages[0] = helpers.TestCorrectionNotification
		notificationMessages[1] = helpers.TestGradedNotification
	}

	return getNotificationsCompletedTests(session, tokenInfo, notificationMessages)
}

func GetTests(session neo4j.Session, path string, token string, testID int, searchString string, singleTest bool) ([]repositories.CompletedTest, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return []repositories.CompletedTest{}, helpers.InvalidTokenError(path, err)
	}

	if tokenInfo.Label == repositories.TeacherLabel {
		if testID != helpers.EmptyIntParameter && !singleTest {
			return getAllCompletedTestsForTeacher(session, testID)
		} else if testID != helpers.EmptyIntParameter {
			return getTestForTeacher(session, tokenInfo.ID, testID)
		} else {
			return getAllTestsForTeacher(session, tokenInfo.ID, searchString)
		}
	}

	return getAllCompletedTestsForStudent(session, tokenInfo.ID, searchString, helpers.EmptyStringParameter)
}

func getAllCompletedTestsForStudent(session neo4j.Session, studentID int, searchString string, subject string) ([]repositories.CompletedTest, error) {
	extraCondition := ""
	extraConditionSearch := ""
	if searchString != helpers.EmptyStringParameter {
		extraConditionSearch = fmt.Sprintf(`
			CALL db.index.fulltext.queryNodes('testsAndSubjects', '%s~')
			YIELD node, score
			WITH collect({name:node.name}) AS rows
			UNWIND rows AS row
			WITH distinct(row.name) AS name
		`, searchString)

		extraCondition = "AND (subj.name = name OR t.name = name)"
	} else if subject != helpers.EmptyStringParameter {
		extraCondition = fmt.Sprintf(`
			AND subj.name = '%s' 
		`, subject)
	}

	query := fmt.Sprintf(` %s 
		MATCH (g:Group)<-[sg:MEMBER_OF]-(s:Student)-[st:COMPLETED]->(t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE s.ID = $studentID %s 
		RETURN s.ID, s.email, s.firstName, s.lastName, g.gID, 
				t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, count(st) as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName, 
				st.testImage, st.gradedTestImage, st.grade, st.timestamp, st.correctedGrade, st.correctedGradeTimestamp, st.notificationMessage, st.feedback, st.answers 
	`, extraConditionSearch, extraCondition)

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest

		fmt.Printf("query: %s\n", query)

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
	extraConditionSearch := ""
	if searchString != helpers.EmptyStringParameter {
		extraConditionSearch = fmt.Sprintf(`
			CALL db.index.fulltext.queryNodes('testsAndSubjects', '%s~')
			YIELD node, score
			WITH collect({name:node.name}) AS rows
			UNWIND rows AS row
			WITH distinct(row.name) AS name
		`, searchString)

		extraCondition = "AND (subj.name = name OR t.name = name)"
	}

	query := fmt.Sprintf(` %s 
		MATCH (p:Teacher)<-[tp:ADDED_BY]-(t:Test)-[ts:BELONGS_TO]->(subj:Subject) 
			OPTIONAL MATCH (Student)-[st:COMPLETED]->(t:Test) 
		WITH p, tp, t, ts, subj, st 
		WHERE p.ID = $teacherID %s
		RETURN t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, count(st) as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName
	`, extraConditionSearch, extraCondition)

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest

		fmt.Printf("query: %s\n", query)

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

func getTestForTeacher(session neo4j.Session, teacherID int, testID int) ([]repositories.CompletedTest, error) {
	query := `
		MATCH (t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE p.ID = $teacherID AND t.testID = $testID 
		RETURN t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, 0 as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName
	`

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, map[string]interface{}{
			"teacherID": teacherID,
			"testID":    testID,
		})
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
				st.testImage, st.gradedTestImage, st.grade, st.timestamp, st.correctedGrade, st.correctedGradeTimestamp, st.notificationMessage, st.feedback, st.answers 
	`

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest

		fmt.Printf("query: %s\n", query)

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

func getNotificationsCompletedTests(session neo4j.Session, tokenInfo repositories.TokenInfo, messages []string) ([]repositories.CompletedTest, error) {
	nodePrefix := "p"
	if tokenInfo.Label == repositories.StudentLabel {
		nodePrefix = "s"
	}

	messagesString, err := json.Marshal(messages)
	if err != nil {
		return []repositories.CompletedTest{}, err
	}
	query := fmt.Sprintf(`
		MATCH (g:Group)<-[sg:MEMBER_OF]-(s:Student)-[st:COMPLETED]->(t:Test)-[ts:BELONGS_TO]->(subj:Subject), (t:Test)-[tp:ADDED_BY]->(p:Teacher) 
		WHERE %s.ID = $ID 
			AND st.notificationMessage IN %s
		RETURN s.ID, s.email, s.firstName, s.lastName, g.gID, 
				t.testID, subj.name, t.name, t.nrQuestions, t.nrAnswers, t.points, t.exOfficio, t.multipleAnswersAllowed, 
					t.enablePartialScoring, t.mandatoryToPass, t.template, count(st) as nrTestsGraded, t.answers, 
					p.ID, p.email, p.firstName, p.lastName, 
				st.testImage, st.gradedTestImage, st.grade, st.timestamp, st.correctedGrade, st.correctedGradeTimestamp, st.notificationMessage, st.feedback, st.answers 
	`, nodePrefix, messagesString)

	testResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		var results []repositories.CompletedTest

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, map[string]interface{}{"ID": tokenInfo.ID})
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
	answers, err := helpers.GetStringParameterFromQuery(record, "st.answers", true, false)
	if err != nil {
		return repositories.CompletedTest{}, err
	}
	mapAnswers, err := helpers.GetAnswerMapFromNeo4jString(answers)
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
		Answers:                 mapAnswers,
	}, nil
}

func getTestFromTestQuery(record neo4j.Record) (repositories.Test, error) {
	teacher, err := getTeacherFromQuery(record)
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
	answers, err := helpers.GetAnswerMapFromQuery(record, "t.answers", true, false)
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
	user, err := getUserFromQuery(record, "s")
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

func getNextNodeID(session neo4j.Session, label string, IDProperty string) (int, error) {
	query := fmt.Sprintf("MATCH (n:%s) RETURN max(n.%s) + 1 as next", label, IDProperty)
	params := map[string]interface{}{}

	nextID, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return 0, err
		}
		for records.Next() {
			nextID, err := helpers.GetIntParameterFromQuery(records.Record(), "next", true, true)
			if err != nil {
				return 0, err
			}

			return nextID, nil
		}

		return 0, nil
	})

	if err != nil {
		return 0, err
	}

	return nextID.(int), nil
}
