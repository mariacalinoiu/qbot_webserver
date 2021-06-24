package datasources

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

const (
	StudentLabel = "Student"
	TeacherLabel = "Teacher"
)

func GetTokenInfo(session neo4j.Session, token string) (repositories.TokenInfo, error) {
	query := `
		MATCH (n) 
		WHERE (n:Student OR n:Teacher) AND n.token = $token 
		RETURN n.ID AS userID, labels(n) AS type
	`
	tokenQueryResults, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, map[string]interface{}{"token": token})
		if err != nil {
			return repositories.TokenInfo{}, err
		}

		for records.Next() {
			record := records.Record()
			resultID, err := helpers.GetIntParameterFromQuery(record, "userID", true, true)
			if err != nil {
				return repositories.TokenInfo{}, err
			}
			resultType, ok := record.Get("type")
			if !ok {
				return repositories.TokenInfo{}, fmt.Errorf("'type' not found in query result")
			}

			return repositories.TokenInfo{
				ID:    resultID,
				Label: helpers.GetStringSliceFromInterfaceSlice(resultType.([]interface{}))[0],
			}, nil
		}

		return repositories.TokenInfo{}, fmt.Errorf("'userID' not found")
	})

	if err != nil {
		return repositories.TokenInfo{}, err
	}

	return tokenQueryResults.(repositories.TokenInfo), nil
}

func DeleteUser(session neo4j.Session, path string, token string) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return helpers.InvalidTokenError(path, err)
	}

	query := `
		MATCH (s:Student) 
		WHERE s.ID = $ID
		DETACH DELETE s
	`
	if tokenInfo.Label == TeacherLabel {
		query = `
			MATCH (p:Teacher) 
			WHERE p.ID = $ID
			DETACH DELETE s
		`
	}

	params := map[string]interface{}{
		"ID": tokenInfo.ID,
	}

	return helpers.WriteTX(session, query, params)
}

func AddUser(session neo4j.Session, userType string, user interface{}) (repositories.Item, error) {
	var err error
	token := helpers.GenerateToken(20)
	if userType == StudentLabel {
		err = addStudent(session, user, token)
	}

	err = addTeacher(session, user, token)
	if err != nil {
		return repositories.Item{}, err
	}

	return repositories.Item{Name: token}, nil
}

func GetUser(session neo4j.Session, path string, token string) (interface{}, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return repositories.User{}, helpers.InvalidTokenError(path, err)
	}

	if tokenInfo.Label == StudentLabel {
		return getStudent(session, tokenInfo, token)
	}

	return getTeacher(session, tokenInfo, token)
}

func addStudent(session neo4j.Session, user interface{}, token string) error {
	student := user.(repositories.Student)
	queryPrefix := ""
	studentID := student.ID
	var err error
	if studentID != 0 {
		queryPrefix = `
			MATCH (s:Student {ID:$studentID}) 
		`
	} else {
		studentID, err = getNextNodeID(session, "Student", "ID")
		if err != nil {
			return err
		}

		queryPrefix = `
			CREATE (s:Student {ID:$studentID}) 
		`
	}

	encryptedPassword, err := helpers.EncryptPassword(student.Password)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		%s 
		SET s.year = '%s', s.email=$email, s.firstName=$firstName, s.lastName=$lastName, s.password=$password, s.token=$token 
	`, queryPrefix, student.Year)

	params := map[string]interface{}{
		"studentID": studentID,
		"email":     student.Email,
		"firstName": student.FirstName,
		"lastName":  student.LastName,
		"password":  encryptedPassword,
		"token":     token,
	}

	err = helpers.WriteTX(session, query, params)
	if err != nil {
		return err
	}

	query = `
		MATCH (s:Student {ID:$studentID}), (g:Group {gID:$gID}) 
		MERGE (s)-[r:MEMBER_OF]->(g) 
	`
	params = map[string]interface{}{
		"studentID": studentID,
		"gID":       student.Group,
	}

	return helpers.WriteTX(session, query, params)
}

func addTeacher(session neo4j.Session, user interface{}, token string) error {
	professor := user.(repositories.Professor)
	queryPrefix := ""
	teacherID := professor.ID
	var err error
	if teacherID != 0 {
		queryPrefix = `
			MATCH (p:Teacher {ID:$teacherID}) 
		`
	} else {
		teacherID, err = getNextNodeID(session, "Teacher", "ID")
		if err != nil {
			return err
		}

		queryPrefix = `
			CREATE (p:Teacher {ID:$teacherID}) 
		`
	}

	encryptedPassword, err := helpers.EncryptPassword(professor.Password)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		%s 
		SET p.email=$email, p.firstName=$firstName, p.lastName=$lastName, p.password=$password, p.token=$token 
	`, queryPrefix)

	params := map[string]interface{}{
		"teacherID": teacherID,
		"email":     professor.Email,
		"firstName": professor.FirstName,
		"lastName":  professor.LastName,
		"password":  encryptedPassword,
		"token":     token,
	}

	err = helpers.WriteTX(session, query, params)
	if err != nil {
		return err
	}

	query = fmt.Sprintf(`
		MATCH (p:Teacher {ID:$teacherID}), (f:Faculty {name:'%s'}) 
		MERGE (p)-[r:AFILLIATED_TO]->(f) 
	`, professor.Faculty)

	params = map[string]interface{}{
		"teacherID": teacherID,
	}

	return helpers.WriteTX(session, query, params)
}

func getTeacher(session neo4j.Session, tokenInfo repositories.TokenInfo, token string) (interface{}, error) {
	query := `
		MATCH (p:Teacher)-[:AFILLIATED_TO]->(f:Faculty), (t:Test)-[c:ADDED_BY]->(p:Teacher)
		WHERE p.ID = $pID
		RETURN p.ID, p.email, p.password, p.firstName, p.lastName, f.name, count(c) as nrTests
	`
	params := map[string]interface{}{
		"pID": tokenInfo.ID,
	}

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return repositories.Professor{}, err
		}
		var results repositories.Professor
		for records.Next() {
			teacher, err := getTeacherFromQuery(records.Record())
			if err != nil {
				return repositories.Professor{}, err
			}
			teacher.User.ID = tokenInfo.ID
			teacher.User.Token = token
			teacher.User.Type = tokenInfo.Label

			return teacher, nil
		}

		return results, nil
	})
	if err != nil {
		return repositories.Professor{}, err
	}

	teacher := result.(repositories.Professor)

	teacherSubjects, err := getSubjectsForUser(session, tokenInfo)
	teacher.Subjects = teacherSubjects

	return teacher, nil
}

func getStudent(session neo4j.Session, tokenInfo repositories.TokenInfo, token string) (interface{}, error) {
	query := `
		MATCH (s:Student)-[:MEMBER_OF]->(g:Group)-[:HAS_SPECIALIZATION]->(spec:Specialization)-[:IN_FACULTY]->(f:Faculty), 
			(s:Student)-[c:COMPLETED]->(t:Test)
		WHERE s.ID = $sID
		RETURN s.ID, s.email, s.password, s.firstName, s.lastName, s.year, f.name, spec.name, g.gID, 
			count(c) as nrTestsCompleted, toInteger(avg(c.grade * 1.0 / t.points) * 100) as averageGrade
	`
	params := map[string]interface{}{
		"sID": tokenInfo.ID,
	}

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return repositories.Student{}, err
		}
		var results repositories.Student
		for records.Next() {
			record := records.Record()
			user, err := getUserFromQuery(record, "s")
			if err != nil {
				return repositories.Student{}, err
			}
			user.ID = tokenInfo.ID
			user.Token = token
			user.Type = tokenInfo.Label

			student, err := getStudentFromQuery(record, user)
			if err != nil {
				return repositories.Student{}, err
			}

			return student, nil
		}

		return results, nil
	})
	if err != nil {
		return repositories.Student{}, err
	}

	student := result.(repositories.Student)

	studentSubjects, err := getSubjectsForUser(session, tokenInfo)
	student.Subjects = studentSubjects

	return student, nil
}

func getSubjectsForUser(session neo4j.Session, tokenInfo repositories.TokenInfo) ([]string, error) {
	query := `
		MATCH (s:Student)-[:ENROLLED_IN]->(subj:Subject) 
		WHERE s.ID = $ID 
		RETURN subj.name 
	`
	if tokenInfo.Label == TeacherLabel {
		query = `
			MATCH (p:Teacher)-[:TEACHES]->(subj:Subject) 
			WHERE p.ID = $ID 
			RETURN subj.name 
		`
	}
	params := map[string]interface{}{
		"ID": tokenInfo.ID,
	}

	subjects, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return []string{}, err
		}
		var results []string
		for records.Next() {
			name, err := helpers.GetStringParameterFromQuery(records.Record(), "subj.name", true, true)
			if err != nil {
				return []string{}, err
			}

			results = append(results, name)
		}

		return results, nil
	})
	if err != nil {
		return []string{}, err
	}

	return subjects.([]string), nil
}

func getTeacherFromQuery(record neo4j.Record) (repositories.Professor, error) {
	user, err := getUserFromQuery(record, "p")
	if err != nil {
		return repositories.Professor{}, err
	}
	nrTests, err := helpers.GetIntParameterFromQuery(record, "nrTests", false, false)
	if err != nil {
		return repositories.Professor{}, err
	}

	return repositories.Professor{
		User:    user,
		NrTests: nrTests,
	}, nil
}

func getStudentFromQuery(record neo4j.Record, user repositories.User) (repositories.Student, error) {
	year, err := helpers.GetStringParameterFromQuery(record, "s.year", true, true)
	if err != nil {
		return repositories.Student{}, err
	}
	specialization, err := helpers.GetStringParameterFromQuery(record, "spec.name", true, true)
	if err != nil {
		return repositories.Student{}, err
	}
	group, err := helpers.GetIntParameterFromQuery(record, "g.gID", true, true)
	if err != nil {
		return repositories.Student{}, err
	}
	nrTestsCompleted, err := helpers.GetIntParameterFromQuery(record, "nrTestsCompleted", true, true)
	if err != nil {
		return repositories.Student{}, err
	}
	averageGrade, err := helpers.GetIntParameterFromQuery(record, "averageGrade", true, true)
	if err != nil {
		return repositories.Student{}, err
	}

	return repositories.Student{
		User:           user,
		Year:           year,
		Specialization: specialization,
		Group:          group,
		NrTestsTaken:   nrTestsCompleted,
		AverageGrade:   averageGrade,
	}, nil
}

func getUserFromQuery(record neo4j.Record, nodeName string) (repositories.User, error) {
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
	lastName, err := helpers.GetStringParameterFromQuery(record, fmt.Sprintf("%s.lastName", nodeName), true, true)
	if err != nil {
		return repositories.User{}, err
	}
	faculty, err := helpers.GetStringParameterFromQuery(record, "f.name", false, false)
	if err != nil {
		return repositories.User{}, err
	}

	return repositories.User{
		ID:        ID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		Faculty:   faculty,
	}, nil
}
