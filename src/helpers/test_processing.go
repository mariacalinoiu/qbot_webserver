package handlers

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/neo4j/neo4j-go-driver/neo4j"

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
)

func GenerateTestTemplate(test repositories.Test) (string, error) {
	// TODO generate template and save it to S3

	filename := strings.ReplaceAll(fmt.Sprintf("%s_%s.pdf", test.Subject, test.Name), " ", "_")

	test.NrQuestions = 36
	test.NrAnswerOptions = 5

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

	_ = pdf.OutputFileAndClose(filename)
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
