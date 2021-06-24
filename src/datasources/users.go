package datasources

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/neo4j"

	helpers "qbot_webserver/src/helpers"
	"qbot_webserver/src/repositories"
)

const tokenLength = 20

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

func GetTokenFromEmailAndPassword(session neo4j.Session, email string, password string) (string, error) {
	var token string
	query := fmt.Sprintf(`
		MATCH (n) 
		WHERE (n:Student OR n:Teacher) AND n.email = $email AND n.password = '%s'
		RETURN n.token
	`, password)
	params := map[string]interface{}{
		"email": email,
	}

	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return helpers.EmptyStringParameter, fmt.Errorf("no user with these credentials")
		}

		for records.Next() {
			record := records.Record()
			token, err := helpers.GetStringParameterFromQuery(record, "token", false, false)
			if err != nil {
				return helpers.EmptyStringParameter, nil
			}

			return token, nil
		}

		return helpers.EmptyStringParameter, fmt.Errorf("no user with these credentials")
	})
	if err != nil {
		return helpers.EmptyStringParameter, err
	}

	token = result.(string)
	if token == helpers.EmptyStringParameter {
		token = helpers.GenerateToken(tokenLength)

		query := fmt.Sprintf(`
			MATCH (n) 
			WHERE (n:Student OR n:Teacher) AND n.email = $email AND n.password = '%s'
			SET n.token=$token
		`, password)
		params = map[string]interface{}{
			"email": email,
			"token": token,
		}

		err = helpers.WriteTX(session, query, params)
		if err != nil {
			return helpers.EmptyStringParameter, err
		}
	}

	return token, nil
}

func DeleteToken(session neo4j.Session, path string, token string) error {
	_, err := GetTokenInfo(session, token)
	if err != nil {
		return helpers.InvalidTokenError(path, err)
	}

	query := `
		MATCH (n) 
		WHERE (n:Student OR n:Teacher) AND n.token = $token 
		REMOVE n.token
	`
	params := map[string]interface{}{
		"token": token,
	}

	return helpers.WriteTX(session, query, params)
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
	if tokenInfo.Label == repositories.TeacherLabel {
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

func ChangePassword(session neo4j.Session, path string, token string, oldPassword string, newPassword string) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return helpers.InvalidTokenError(path, err)
	}

	query := fmt.Sprintf(`
		MATCH (s:Student) 
		WHERE s.ID = $ID AND s.password='%s'
		SET s.password='%s'
	`, oldPassword, newPassword)
	if tokenInfo.Label == repositories.TeacherLabel {
		query = fmt.Sprintf(`
			MATCH (p:Teacher) 
			WHERE p.ID = $ID AND p.password='%s'
			SET p.password='%s'
		`, oldPassword, newPassword)
	}

	params := map[string]interface{}{
		"ID": tokenInfo.ID,
	}

	return helpers.WriteTX(session, query, params)
}

func SetSubjectsForUser(session neo4j.Session, path string, token string, subjects []string) error {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return helpers.InvalidTokenError(path, err)
	}

	query := `
		MATCH (s:Student {ID:$ID})-[r:ENROLLED_IN]->(subj:Subject) 
		DELETE r
	`
	if tokenInfo.Label == repositories.TeacherLabel {
		query = `
			MATCH (p:Teacher {ID:$ID})-[r:TEACHES]->(subj:Subject) 
			DELETE r
		`
	}
	params := map[string]interface{}{
		"ID": tokenInfo.ID,
	}

	err = helpers.WriteTX(session, query, params)
	if err != nil {
		return err
	}

	for _, subject := range subjects {
		query = fmt.Sprintf(`
			MATCH (s:Student {ID:$ID}), (subj:Subject {name:'%s'}) 
			MERGE (s)-[r:ENROLLED_IN]->(subj)
		`, subject)
		if tokenInfo.Label == repositories.TeacherLabel {
			query = fmt.Sprintf(`
				MATCH (p:Teacher {ID:$ID}), (subj:Subject {name:'%s'}) 
				MERGE (p)-[r:TEACHES]->(subj)
			`, subject)
		}
		params = map[string]interface{}{
			"ID": tokenInfo.ID,
		}

		err = helpers.WriteTX(session, query, params)
		if err != nil {
			return err
		}
	}

	return nil
}

func AddUser(session neo4j.Session, userType string, user interface{}) (repositories.Item, error) {
	var err error
	token := helpers.GenerateToken(tokenLength)
	if userType == repositories.StudentType {
		err = addStudent(session, user, token)
	} else {
		err = addTeacher(session, user, token)
	}
	if err != nil {
		return repositories.Item{}, err
	}

	return repositories.Item{Name: token}, nil
}

func GetUserByEmailAndPassword(session neo4j.Session, path string, email string, password string) (interface{}, error) {
	token, err := GetTokenFromEmailAndPassword(session, email, password)
	if err != nil || token == helpers.EmptyStringParameter {
		return repositories.User{}, helpers.InvalidTokenError(path, err)
	}

	return GetUser(session, path, token)
}

func GetUser(session neo4j.Session, path string, token string) (interface{}, error) {
	tokenInfo, err := GetTokenInfo(session, token)
	if err != nil {
		return repositories.User{}, helpers.InvalidTokenError(path, err)
	}

	if tokenInfo.Label == repositories.StudentLabel {
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

	query := fmt.Sprintf(`
		%s 
		SET s.year = '%s', s.email=$email, s.firstName=$firstName, s.lastName=$lastName, s.password=$password, s.token=$token 
	`, queryPrefix, student.Year)

	params := map[string]interface{}{
		"studentID": studentID,
		"email":     student.Email,
		"firstName": student.FirstName,
		"lastName":  student.LastName,
		"password":  student.Password,
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

	query := fmt.Sprintf(`
		%s 
		SET p.email=$email, p.firstName=$firstName, p.lastName=$lastName, p.password=$password, p.token=$token 
	`, queryPrefix)

	params := map[string]interface{}{
		"teacherID": teacherID,
		"email":     professor.Email,
		"firstName": professor.FirstName,
		"lastName":  professor.LastName,
		"password":  professor.Password,
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
		MATCH (p:Teacher)-[:AFILLIATED_TO]->(f:Faculty) 
		WHERE p.ID = $pID
		RETURN p.ID, p.email, p.password, p.firstName, p.lastName, f.name 
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
			teacher.User.Type = repositories.TeacherType

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

	query = `
		MATCH (t:Test)-[c:ADDED_BY]->(p:Teacher)
		WHERE p.ID = $pID
		RETURN count(c) as nrTests
	`
	params = map[string]interface{}{
		"pID": tokenInfo.ID,
	}

	result, err = session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return repositories.Professor{}, err
		}
		var results repositories.Professor
		for records.Next() {
			teacher, err := getTeacherStatsFromQuery(records.Record())
			if err != nil {
				return repositories.Professor{}, err
			}
			teacher.User.ID = tokenInfo.ID
			teacher.User.Token = token
			teacher.User.Type = repositories.TeacherType

			return teacher, nil
		}

		return results, nil
	})
	if err != nil {
		return repositories.Professor{}, err
	}

	teacherStats := result.(repositories.Professor)
	teacher.NrTests = teacherStats.NrTests

	return teacher, nil
}

func getStudent(session neo4j.Session, tokenInfo repositories.TokenInfo, token string) (interface{}, error) {
	query := `
		MATCH (s:Student)-[:MEMBER_OF]->(g:Group)-[:HAS_SPECIALIZATION]->(spec:Specialization)-[:IN_FACULTY]->(f:Faculty) 
		WHERE s.ID = $sID
		RETURN s.ID, s.email, s.password, s.firstName, s.lastName, s.year, f.name, spec.name, g.gID 
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
			user.Type = repositories.StudentType

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

	query = `
		MATCH (s:Student)-[c:COMPLETED]->(t:Test) 
		WHERE s.ID = $sID 
		RETURN count(c) as nrTestsCompleted, toInteger(avg(c.grade * 1.0 / t.points) * 100) as averageGrade 
	`
	params = map[string]interface{}{
		"sID": tokenInfo.ID,
	}

	result, err = session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {

		fmt.Printf("query: %s\n", query)

		records, err := tx.Run(query, params)
		if err != nil {
			return repositories.Student{}, err
		}
		var results repositories.Student
		for records.Next() {
			record := records.Record()
			student, err := getStudentStatsFromQuery(record)
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

	statsForStudent := result.(repositories.Student)

	student.AverageGrade = statsForStudent.AverageGrade
	student.NrTestsTaken = statsForStudent.NrTestsTaken

	return student, nil
}

func getSubjectsForUser(session neo4j.Session, tokenInfo repositories.TokenInfo) ([]string, error) {
	query := `
		MATCH (s:Student)-[:ENROLLED_IN]->(subj:Subject) 
		WHERE s.ID = $ID 
		RETURN subj.name 
	`
	if tokenInfo.Label == repositories.TeacherLabel {
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

	return repositories.Professor{
		User: user,
	}, nil
}

func getTeacherStatsFromQuery(record neo4j.Record) (repositories.Professor, error) {
	nrTests, err := helpers.GetIntParameterFromQuery(record, "nrTests", false, false)
	if err != nil {
		return repositories.Professor{}, err
	}

	return repositories.Professor{
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

	return repositories.Student{
		User:           user,
		Year:           year,
		Specialization: specialization,
		Group:          group,
	}, nil
}

func getStudentStatsFromQuery(record neo4j.Record) (repositories.Student, error) {
	nrTestsCompleted, err := helpers.GetIntParameterFromQuery(record, "nrTestsCompleted", true, true)
	if err != nil {
		return repositories.Student{}, err
	}
	averageGrade, err := helpers.GetIntParameterFromQuery(record, "averageGrade", false, false)
	if err != nil {
		return repositories.Student{}, err
	}

	return repositories.Student{
		NrTestsTaken: nrTestsCompleted,
		AverageGrade: averageGrade,
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
