package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	gym "github.com/ryankscott/go_gymclass"
)

var config *gym.Config

func main() {
	var err error
	config, err = gym.NewConfig()
	if err != nil {
		log.Fatal("Unable to get gym configuration")
	}

	// Create a new router for requests
	router := mux.NewRouter().StrictSlash(true)

	// Register routers
	router.HandleFunc("/class/", Class)
	router.HandleFunc("/search/", Search)
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./ui/")))

	// Start the http server
	log.Fatal(http.ListenAndServe(":9000", router))

}

// Class returns the classes for a particular gym class at a gym between times
// /class/?gym="city"&name="cx"&after=2015-07-01T11:20&before=2015-07-01T13:30&limit=10
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
	searchGym := gym.GetGymByName(params.Get("gym"))
	var query = gym.GymQuery{
		Gym:    searchGym,
		Class:  params.Get("name"),
		Before: beforeTime,
		After:  afterTime,
		Limit:  resultLimit,
	}

	foundClasses, err := gym.QueryClasses(query, config)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
		return
	}

	classes, err := json.Marshal(foundClasses)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(classes))
	return
}

// Search returns the classes based on a parsed query in natural english
func Search(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var params = r.URL.Query()
	query := params.Get("q")
	if query == "" {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Unable to get search query")
		return
	}
	foundClasses, err := gym.QueryClassesByName(query, config)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
		return
	}
	classes, err := json.Marshal(foundClasses)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, string(classes))
	return

}
