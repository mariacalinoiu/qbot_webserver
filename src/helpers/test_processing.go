package handlers

import (
	"log"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	"qbot_webserver/src/repositories"
)

const (
	GradingErrorNotification   = "Test requires correction!"
	TestGradedNotification     = "Test has been graded!"
	TestCorrectionNotification = "Test has been corrected!"
)

func GenerateTestTemplate(test repositories.Test) (string, error) {
	// TODO generate template and save it to S3

	return "https://i.pinimg.com/564x/24/4c/8b/244c8b25406a92dfba3fbfa1e803d824.jpg", nil
}

func GradeTestImage(logger *log.Logger, session neo4j.Session, teacherID int, test repositories.CompletedTest) {
	// TODO save test image to S3, call Python script to grade, save graded image to S3, run query below

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
