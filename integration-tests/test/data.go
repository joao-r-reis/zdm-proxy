package test

import (
	"fmt"

	"github.com/gocql/gocql"
	log "github.com/sirupsen/logrus"
)

// Task is a struct that represents a row in the test db
type Task struct {
	ID        gocql.UUID
	Task      string
	WriteTime int64
	TTL       int
}

// DataIds is the seed id column data
var DataIds []string

// DataTasks is the seed task column data
var DataTasks []string

// DropExistingKeyspace drops TestKeyspace from the specified session
func DropExistingKeyspace(session *gocql.Session) {
	session.Query(fmt.Sprintf("DROP KEYSPACE %s;", TestKeyspace)).Exec()
}

// SeedKeyspace seeds the specified source and dest sessions with TestKeyspace
func SeedKeyspace(source *gocql.Session, dest *gocql.Session) {
	log.Info("Seeding keyspace...")
	source.Query(fmt.Sprintf("CREATE KEYSPACE %s WITH replication = {'class':'SimpleStrategy', 'replication_factor':1};", TestKeyspace)).Exec()
	dest.Query(fmt.Sprintf("CREATE KEYSPACE %s WITH replication = {'class':'SimpleStrategy', 'replication_factor':1};", TestKeyspace)).Exec()
}

// SeedData seeds the specified source and dest sessions with data in data.go
// Currently, this includes DataIds and DataTasks
func SeedData(source *gocql.Session, dest *gocql.Session) {
	log.Info("Seeding tables...")
	// Create the table in source
	err := source.Query(fmt.Sprintf("CREATE TABLE %s.%s(id UUID, task text, PRIMARY KEY(id));", TestKeyspace, TestTable)).Exec()
	if err != nil {
		log.WithError(err).Error(err)
	}

	err = dest.Query(fmt.Sprintf("CREATE TABLE %s.%s(id UUID, task text, PRIMARY KEY(id));", TestKeyspace, TestTable)).Exec()
	if err != nil {
		log.WithError(err).Error(err)
	}

	// Seed the rows
	for i := 0; i < len(DataIds); i++ {
		id, task := DataIds[i], DataTasks[i]
		err = source.Query(fmt.Sprintf("INSERT INTO %s.%s(id, task) VALUES (%s, '%s');", TestKeyspace, TestTable, id, task)).Exec()
		if err != nil {
			log.WithError(err).Error(err)
		}
	}
}

// MapToTask converts a map loaded by Iter.MapScan() into a Task
func MapToTask(row map[string]interface{}) Task {
	return Task{
		ID:        row["id"].(gocql.UUID),
		Task:      row["task"].(string),
		WriteTime: row["w_task"].(int64),
		TTL:       row["l_task"].(int),
	}
}

// UnloadData unloads data from the specified source session
func UnloadData(source *gocql.Session) []Task {
	unloadedData := make([]Task, 0)

	query := `SELECT id, task, WRITETIME(task) as w_task, TTL(task) as l_task FROM cloudgate_test.tasks;`
	itr := source.Query(query).Iter()

	for {
		row := make(map[string]interface{})
		if !itr.MapScan(row) {
			break
		}

		unloadedData = append(unloadedData, MapToTask(row))
	}
	return unloadedData
}

// LoadData loads the given data into the specified destination session
func LoadData(dest *gocql.Session, unloadedData []Task) {
	query := "BEGIN BATCH "
	for _, task := range unloadedData {
		query += fmt.Sprintf("INSERT INTO test.tasks(id, task) VALUES (%s, '%s') USING TIMESTAMP %d AND TTL %d; ",
			task.ID, task.Task, task.WriteTime, task.TTL)
	}
	query += "APPLY BATCH;"
	err := dest.Query(query).Exec()
	if err != nil {
		log.Fatal(err)
	}
}
