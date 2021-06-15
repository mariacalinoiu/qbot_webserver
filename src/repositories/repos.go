package repositories

const (
	Token          = "token"
	Faculty        = "faculty"
	Specialization = "specialization"
	Subject        = "subject"
	TestID         = "test"
	StudentID      = "studentId"
	NewGrade       = "newGrade"
	Feedback       = "feedback"
	ForUserOnly    = "forUserOnly"
	Search         = "search"
)

type SpinnerItem struct {
	Name string `json:"name"`
}

type TokenItem struct {
	Token string `json:"token"`
}

type ResponseItem struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type TokenInfo struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type User struct {
	ID        int      `json:"id"`
	Type      string   `json:"type"`
	Token     string   `json:"token"`
	Email     string   `json:"email"`
	Password  string   `json:"password"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Faculty   string   `json:"faculty"`
	Subjects  []string `json:"subjects"`
}

type Professor struct {
	User
	NrTests int `json:"nrTests"`
}

type Student struct {
	User
	Year           string `json:"year"`
	Specialization string `json:"specialization"`
	Group          int    `json:"group"`
	NrTestsTaken   int    `json:"nrTestsTaken"`
	AverageGrade   int    `json:"averageGrade"`
}

type Test struct {
	ID                     int              `json:"id"`
	Subject                string           `json:"subject"`
	Name                   string           `json:"name"`
	NrQuestions            int              `json:"nrQuestions"`
	NrAnswerOptions        int              `json:"nrAnswerOptions"`
	TotalPoints            int              `json:"totalPoints"`
	ExOfficioPoints        int              `json:"exOfficioPoints"`
	MultipleAnswersAllowed bool             `json:"multipleAnswersAllowed"`
	EnablePartialScoring   bool             `json:"enablePartialScoring"`
	MandatoryToPass        bool             `json:"mandatoryToPass"`
	TemplateImageURL       string           `json:"templateImageURL"`
	NrTestsGraded          int              `json:"nrTestsGraded"`
	Teacher                Professor        `json:"professor"`
	CorrectAnswers         map[int][]string `json:"correctAnswers"`
}

type CompletedTest struct {
	Test
	TestImageURL            string  `json:"testImageURL"`
	GradedTestImageURL      string  `json:"gradedTestImageURL"`
	Grade                   int     `json:"grade"`
	GradeTimestamp          int     `json:"gradeTimestamp"`
	CorrectedGrade          int     `json:"correctedGrade"`
	CorrectedGradeTimestamp int     `json:"correctedGradeTimestamp"`
	NotificationMessage     string  `json:"notificationMessage"`
	ImageBytes              string  `json:"imageBytes"`
	Feedback                string  `json:"feedback"`
	Author                  Student `json:"student"`
}

type Objective struct {
	ID             int             `json:"id"`
	Subject        string          `json:"subject"`
	TargetGrade    int             `json:"targetGrade"`
	StartTimestamp int             `json:"startTimestamp"`
	EndTimestamp   int             `json:"endTimestamp"`
	Tests          []CompletedTest `json:"tests"`
}
