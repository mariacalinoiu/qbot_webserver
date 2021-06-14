package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
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

	filename := strings.ReplaceAll(test.Name, " ", "_")

	header := make([]string, test.NrAnswerOptions+1)
	gridSizes := make([]uint, test.NrAnswerOptions+1)
	header[0] = "Nr."
	gridSizes[0] = 1
	for i := 1; i <= test.NrAnswerOptions; i++ {
		//header[i] = "|"
		//gridSizes[i] = 1
		//header[i+1] = string(rune('A' + i/2))
		//gridSizes[i+1] = 1
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

	fmt.Printf("%+v\n", contents)
	fmt.Printf("%+v\n", gridSizes)

	m := pdf.NewMaroto(consts.Portrait, consts.A4)
	m.SetPageMargins(10, 15, 10)

	m.RegisterHeader(func() {
		m.Row(65, func() {
			m.Col(4, func() {
				m.Text("First Name:", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   5,
				})
				m.Text("Last Name:", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   12,
				})
				m.Text("University e-mail address:", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   19,
				})
				m.Text("Year:", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   26,
				})
				m.Text("Group:", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   33,
				})
				m.Text("Specialization:", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   40,
				})
			})
			m.Col(5, func() {
				m.Text("_____________________________________", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   5,
				})
				m.Text("_____________________________________", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   12,
				})
				m.Text("_____________________________________", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   19,
				})
				m.Text("_____________________________________", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   26,
				})
				m.Text("_____________________________________", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   33,
				})
				m.Text("_____________________________________", props.Text{
					Size:  12,
					Style: consts.Bold,
					Align: consts.Left,
					Top:   40,
				})
			})
		})

		m.Row(40, func() {
			m.Col(0, func() {
				m.Text(test.Name, props.Text{
					Size:  18,
					Style: consts.Bold,
					Align: consts.Center,
					Top:   4,
				})
				m.Text(test.Subject, props.Text{
					Size:  15,
					Align: consts.Center,
					Top:   15,
				})
			})
		})
	})

	m.TableList(header, contents, props.TableList{
		ContentProp: props.TableListContent{
			Family:    consts.Arial,
			GridSizes: gridSizes,
		},
		HeaderProp: props.TableListContent{
			Family:    consts.Arial,
			GridSizes: gridSizes,
		},
		Align: consts.Right,
		Line:  true,
	})

	_ = m.OutputFileAndClose(filename)
	//if err != nil {
	//	return "", err
	//}

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
