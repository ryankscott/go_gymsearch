package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/PuloV/ics-golang"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	//	"html"
	"log"
	"net/http"
	"sort"
	"time"
)

var gymIds = map[string]string{
	"city":      "96382586-e31c-df11-9eaa-0050568522bb",
	"britomart": "744366a6-c70b-e011-87c7-0050568522bb",
	"takapuna":  "98382586-e31c-df11-9eaa-0050568522bb",
	"newmarket": "b6aa431c-ce1a-e511-a02f-0050568522bb",
}

type GymClass struct {
	Gym           string    `json:"gym" db:"gym"`
	Name          string    `json:"name" db:"class"`
	Location      string    `json:"location" db:"location"`
	StartDateTime time.Time `json:"startdatetime" db:"start_datetime"`
	EndDateTime   time.Time `json:"enddatetime" db:"end_datetime"`
}

type GymQuery struct {
	Gym    string
	Name   string
	Before time.Time
	After  time.Time
	Limit  string
}

// ByStartDateTime implements sort.Interface for []GymClass based on the StartDateTime
type ByStartDateTime []GymClass

func (a ByStartDateTime) Len() int           { return len(a) }
func (a ByStartDateTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByStartDateTime) Less(i, j int) bool { return a[i].StartDateTime.Before(a[j].StartDateTime) }

var db *sql.DB

func main() {

	// Try open the DB
	var err error
	db, err = sql.Open("sqlite3", "./gym.db?charset=utf8&parseTime=true")
	if err != nil {
		log.Fatal("Failed to open database")
	}

	// Create a new router for requests
	router := mux.NewRouter().StrictSlash(true)

	// Register a router for the class endpoint
	router.HandleFunc("/", Class)

	// Start the http server
	log.Fatal(http.ListenAndServe(":9000", router))
}

// /?class="city"&name="cx"&after=2015-07-01T11:20&before=2015-07-01T13:30&limit=10
func Class(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var params = r.URL.Query()

	// Parse the before time
	beforeTime, err := time.Parse(time.RFC3339, params.Get("before"))
	if err != nil {
		log.Printf("Could not parse beforeTime so setting it to a day in advance: %s", err)
		beforeTime = time.Now().AddDate(1, 0, 0)
	}

	// Parse the after time
	afterTime, err := time.Parse(time.RFC3339, params.Get("after"))
	if err != nil {
		log.Printf("Could not parse afterTime so setting it to now: %s", err)
		afterTime = time.Now()
	}

	// Check that beforeTime is greater than afterTime
	if beforeTime.Before(afterTime) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "'before' parameter must be after 'after' parameter")
		return
	}

	// Check the limit parameter
	resultLimit := params.Get("limit")
	if resultLimit == "" {
		log.Println("Could not find limit parameter, defaulting to 1000")
		resultLimit = "1000"
	}
	resultNumber, err := strconv.Atoi(resultLimit)
	if err != nil || resultNumber < 1 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Limit must be an integer")
		return
	}

	// TODO: Handle no gym
	var foundQuery = GymQuery{
		Gym:    params.Get("gym"),
		Name:   params.Get("name"),
		Before: beforeTime,
		After:  afterTime,
		Limit:  resultLimit,
	}

	var foundClasses []GymClass
	if isDBCurrent() {
		log.Printf("Database is up to date, so getting info from there")
		foundClasses, err = getClassesFromDB(foundQuery)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("Database is stale, parsing ICS files")
		foundClasses, err = parseClassesFromICS(foundQuery)
		if err != nil {
			log.Fatal(err)
		}
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(foundClasses)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func translateGymClassName(className string) string {
	switch {
	case strings.Contains(strings.ToUpper(className), "RPM"):
		return "RPM"
	case strings.Contains(strings.ToUpper(className), "GRIT STRENGTH"):
		return "GRIT STRENGTH"
	case strings.Contains(strings.ToUpper(className), "GRIT CARDIO"):
		return "GRIT CARDIO"
	case strings.Contains(strings.ToUpper(className), "BODYPUMP"):
		return "BODYPUMP"
	case strings.Contains(strings.ToUpper(className), "BODYBALANCE"):
		return "BODYBALANCE"
	case strings.Contains(strings.ToUpper(className), "BODYATTACK"):
		return "BODYATTACK"
	case strings.Contains(strings.ToUpper(className), "CXWORX"):
		return "CXWORX"
	case strings.Contains(strings.ToUpper(className), "SH'BAM"):
		return "SH'BAM"
	case strings.Contains(strings.ToUpper(className), "BODYCOMBAT"):
		return "BODYCOMBAT"
	case strings.Contains(strings.ToUpper(className), "YOGA"):
		return "YOGA"
	case strings.Contains(strings.ToUpper(className), "GRIT PLYO"):
		return "GRIT PLYO"
	case strings.Contains(strings.ToUpper(className), "BODYJAM"):
		return "BODYJAM"
	case strings.Contains(strings.ToUpper(className), "SPRINT"):
		return "SPRINT"
	case strings.Contains(strings.ToUpper(className), "BODYVIVE"):
		return "BODYVIVE"
	case strings.Contains(strings.ToUpper(className), "BODYSTEP"):
		return "BODYSTEP"
	case strings.Contains(strings.ToUpper(className), "BORN TO MOVE"):
		return "BORN TO MOVE"
	}
	return className
}

func isDBCurrent() bool {
	var lastGymClassString string
	err := db.QueryRow("SELECT MAX(start_datetime) from timetable").Scan(&lastGymClassString)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("Couldn't find any rows")
		return false
	case err != nil:
		log.Fatal(err)
		return false
	default:
		fmt.Println(lastGymClassString)
		lastGymClass, err := time.Parse("2006-01-02 15:04:05Z07:00", lastGymClassString)
		if err != nil {
			log.Fatal(err)
		}
		return time.Now().AddDate(0, 0, 6).Before(lastGymClass)
	}
}

func getClassesFromDB(query GymQuery) ([]GymClass, error) {
	// Prepare the SELECT query
	var err error
	var stmt *sql.Stmt
	stmt, err = db.Prepare("SELECT * FROM timetable WHERE gym LIKE ? AND class LIKE ? and start_datetime > ? and start_datetime < ? limit ?")
	if err != nil {
		fmt.Println("Error preparing query")
	}

	// TODO: Refactor this
	likeGym := "%" + query.Gym + "%"
	likeName := "%" + query.Name + "%"
	rows, err := stmt.Query(
		strings.ToLower(likeGym),
		strings.ToLower(likeName),
		query.After,
		query.Before,
		query.Limit,
	)

	var results []GymClass
	for rows.Next() {
		var result GymClass
		err := rows.Scan(
			&result.Gym,
			&result.Name,
			&result.Location,
			&result.StartDateTime,
			&result.EndDateTime,
		)
		if err != nil {
			log.Fatal(err)
			return nil, errors.New("No records found")
		}
		result.Name = translateGymClassName(result.Name)
		results = append(results, result)
	}

	sort.Sort(ByStartDateTime(results))
	return results, nil
}

func parseClassesFromICS(query GymQuery) ([]GymClass, error) {
	baseURL := "https://www.lesmills.co.nz/timetable-calander.ashx?club="

	var foundClasses = []GymClass{}

	parser := ics.New()
	inputChan := parser.GetInputChan()

	if query.Gym == "" {
		for _, v := range gymIds {
			inputChan <- baseURL + v
		}
	} else {
		inputChan <- baseURL + gymIds[query.Gym]
	}
	parser.Wait()

	cal, err := parser.GetCalendars()
	if err != nil {
		log.Fatal(err)
	}
	var foundClass GymClass
	for _, c := range cal {

		// TODO: Refactor me
		gymID := strings.Split(c.GetUrl(), baseURL)[1]
		var gym string
		for k, v := range gymIds {
			if v == gymID {
				gym = k
			}
		}

		for _, event := range c.GetEvents() {
			foundClass = GymClass{
				Gym:           gym,
				Name:          translateGymClassName(event.GetSummary()),
				Location:      event.GetLocation(),
				StartDateTime: event.GetStart(),
				EndDateTime:   event.GetEnd()}

			persistClassToDB(foundClass)
			// Check the times and name
			if (event.GetStart().Before(query.Before)) &&
				(event.GetStart().After(query.After) &&
					(strings.Contains(strings.ToUpper(event.GetSummary()), strings.ToUpper(query.Name)))) {
				foundClasses = append(foundClasses, foundClass)
			}
		}
	}

	sort.Sort(ByStartDateTime(foundClasses))
	return foundClasses, nil
}

func persistClassToDB(class GymClass) {
	// Try open the DB

	// Create table
	createSQL := `
        CREATE TABLE IF NOT EXISTS timetable (
           gym VARCHAR(9) NOT NULL,
           class VARCHAR(45) NOT NULL,
           location VARCHAR(27) NOT NULL,
           start_datetime DATETIME NOT NULL,
           end_datetime DATETIME NOT NULL);
	`
	var err error
	_, err = db.Exec(createSQL)
	if err != nil {
		log.Fatal(err)
	}

	// Create index
	insertSQL := `
	CREATE UNIQUE INDEX IF NOT EXISTS unique_class ON timetable(gym, location, start_datetime);
`
	_, err = db.Exec(insertSQL)
	if err != nil {
		log.Fatal(err)
	}

	// Prepare insert query
	stmt, err := db.Prepare("INSERT OR IGNORE INTO timetable (gym, class, location, start_datetime, end_datetime) values(?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}

	_, err = stmt.Exec(class.Gym, class.Name, class.Location, class.StartDateTime, class.EndDateTime)
	if err != nil {
		log.Fatal(err)
	}

	return
}
