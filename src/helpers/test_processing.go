package handlers

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

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

func GenerateTestTemplate(test repositories.Test, s3Bucket string, s3Profile string, logger *log.Logger) (string, error) {
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

	return uploadToS3(s3Bucket, s3Profile, filenameJPG, testTemplatesFolder)
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

func uploadToS3(s3Bucket string, s3Profile string, filename string, folder string) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("eu-central-1"),
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
