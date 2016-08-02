package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuloV/ics-golang"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
	"strconv"
	"strings"
	//	"html"
	"log"
	"net/http"
	"sort"
	"time"
)

// TODO: Persist LatLong to dB?

var gymIds = map[string]string{
	"city":      "96382586-e31c-df11-9eaa-0050568522bb",
	"britomart": "744366a6-c70b-e011-87c7-0050568522bb",
	"takapuna":  "98382586-e31c-df11-9eaa-0050568522bb",
	"newmarket": "b6aa431c-ce1a-e511-a02f-0050568522bb",
}

var gymLocations = map[string]string{
	"city":      "-36.8483137,174.6877862",
	"britomart": "-36.845961,174.759604",
	"takapuna":  "-36.787821,174.7679373",
	"newmarket": "-36.8662563,174.7685271",
}

type GymClass struct {
	Gym           string    `json:"gym" db:"gym"`
	Name          string    `json:"name" db:"class"`
	Location      string    `json:"location" db:"location"`
	StartDateTime time.Time `json:"startdatetime" db:"start_datetime"`
	EndDateTime   time.Time `json:"enddatetime" db:"end_datetime"`
	LatLong       string    `json:"latlong"`
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

func initialiseDatabase() {

	// Try open the DB
	var err error
	db, err = sql.Open("sqlite3", "./gym.db?charset=utf8&parseTime=true")
	if err != nil {
		log.Fatal("Failed to open database")
	}

	// Create classes table
	createSQL := `
        CREATE TABLE IF NOT EXISTS timetable (
           gym VARCHAR(9) NOT NULL,
           class VARCHAR(45) NOT NULL,
           location VARCHAR(27) NOT NULL,
           start_datetime DATETIME NOT NULL,
           end_datetime DATETIME NOT NULL);
	`
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
}

func main() {

	initialiseDatabase()

	// Create a new router for requests
	router := mux.NewRouter().StrictSlash(true)

	// Register a router for the class endpoint
	router.HandleFunc("/class/", Class)

	// Register a router for the class endpoint
	router.HandleFunc("/traveltime/", TravelTime)

	// Start the http server
	log.Fatal(http.ListenAndServe(":9000", router))
}

// /traveltime/origin=&destination=?
func TravelTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	c, err := maps.NewClient(maps.WithAPIKey("AIzaSyDxQNzW3Ub5U4UQl0KoJWXx368Qg3lGf3A"))
	// Initialise Google Maps API
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "An unknown error occurred")
		return
	}

	var params = r.URL.Query()
	origin := params.Get("origin")
	if origin == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Must supply a 'origin' parameter")
		return
	}

	destination := params.Get("destination")
	if destination == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Must supply a 'destination' parameter")
		return
	}

	req := &maps.DirectionsRequest{
		Origin:      origin,
		Destination: destination,
	}

	resp, _, err := c.Directions(context.Background(), req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "An unknown error occurred")
	}
	//
	jsonResponse := resp[0].Legs
	finalDuration := jsonResponse[len(jsonResponse)-1].Duration

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%.0f", finalDuration.Minutes())
	return
}

// /class/gym="city"&name="cx"&after=2015-07-01T11:20&before=2015-07-01T13:30&limit=10
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
			log.Fatalf("Failed to get classes from the database %s", err)
		}
	} else {
		log.Printf("Database is stale, parsing ICS files")
		foundClasses, err = parseClassesFromICS(foundQuery)
		if err != nil {
			log.Fatalf("Failed to parse ICS files %s", err)
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
		log.Printf("Couldn't find any rows getting newest class from db")
		return false
	case err != nil:
		log.Printf("Unknown error occured when getting newest class from db: %s", err)
		return false
	default:
		lastGymClass, err := time.Parse("2006-01-02 15:04:05Z07:00", lastGymClassString)
		if err != nil {
			log.Printf("Malformed datetime returned from db when trying to find latest class: %s", err)
			return false
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
			return nil, errors.New("No records found")
		}
		result.Name = translateGymClassName(result.Name)
		result.LatLong = gymLocations[result.Gym]
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

	// Create the URL for the ICS based on the gym
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
		return nil, err
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

		loc, _ := time.LoadLocation("Pacific/Auckland")
		for _, event := range c.GetEvents() {
			// TODO: Find a nicer way of adding the timezone
			start := event.GetStart()
			end := event.GetEnd()
			startDateTime := time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), start.Minute(), start.Second(), 0, loc)
			endDateTime := time.Date(end.Year(), end.Month(), end.Day(), end.Hour(), end.Minute(), end.Second(), 0, loc)

			foundClass = GymClass{
				Gym:           gym,
				Name:          translateGymClassName(event.GetSummary()),
				Location:      event.GetLocation(),
				StartDateTime: startDateTime,
				EndDateTime:   endDateTime,
				LatLong:       gymLocations[gym],
			}

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

func persistClassToDB(class GymClass) bool {
	// Prepare insert query
	stmt, err := db.Prepare("INSERT OR IGNORE INTO timetable (gym, class, location, start_datetime, end_datetime) values(?, ?, ?, ?, ?)")
	if err != nil {
		return false
	}

	_, err = stmt.Exec(class.Gym, class.Name, class.Location, class.StartDateTime, class.EndDateTime)
	if err != nil {
		return false
	}
	return true
}
