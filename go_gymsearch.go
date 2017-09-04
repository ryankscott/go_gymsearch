package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"time"

	mailgun "gopkg.in/mailgun/mailgun-go.v1"

	"log"
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/robfig/cron"
	gym "github.com/ryankscott/go_gymclass"
)

var config *gym.Config

func sendEmail(us []gym.User) error {
	mg := mailgun.NewMailgun("ryankscott.com", "key-39491ebd7a69f1daeefd3c20ac48cb07", "pubkey-62227b10c2d201bb4e635150be69e97b")
	t, err := template.ParseFiles("template.html")
	if err != nil {
		log.Printf("Failed to parse template for email with error %s", err)
		return err
	}

	log.Printf("Preparing to send %d emails\n", len(us))
	for _, u := range us {
		pref, err := gym.QueryUserPreferences(u.ID, config)
		if err != nil {
			log.Printf("Failed to get preferences for user %s", err)
		}

		data := struct {
			Name string
		}{
			Name: u.FirstName,
		}
		var out bytes.Buffer
		err = t.Execute(&out, data)
		if err != nil {
			log.Printf("Failed to execute template - %s\n", err)
			return err
		}

		log.Printf("Sending email to %s\n", u.Email)
		message := mailgun.NewMessage(
			"reminders@ryankscott.com",
			"Don't forget to workout today",
			"Don't forget to workout today",
			u.Email)
		message.SetHtml(out.String())

		resp, _, err := mg.Send(message)
		if err != nil {
			log.Printf("Failed to send message with resp - %s and err - %s\n", resp, err)
			return err
		}
	}
	return nil
}

func getUsersWithoutClasses(config *gym.Config) ([]gym.User, error) {
	ty, tm, td := time.Now().Date()
	contactableUsers := make([]gym.User, 0)
	// Get all Users
	allUsers, err := gym.QueryUsers(config)
	if err != nil {
		log.Printf("Failed to store classes: %s", err)
		return []gym.User{}, err
	}
	// For each user get the last classes
	for _, user := range allUsers {
		stats, err := gym.QueryUserStatistics(user.ID, config)
		if err != nil {
			log.Printf("Failed to get stats for user %s with error %s", user.ID, err)
			continue
		}
		// Check if the last class was today
		cy, cm, cd := stats.LastClassDate.Date()
		if (cy != ty) || (cm != tm) || (cd != td) {
			if user.Email != "" {
				contactableUsers = append(contactableUsers, user)
			}
		}
	}
	return contactableUsers, nil
}

func getAndStoreClasses(config *gym.Config) {
	cityClasses, err := gym.GetClasses(gym.Gyms)
	if err != nil {
		log.Printf("Failed to get classes: %s", err)
	}
	err = gym.StoreClasses(cityClasses, config)
	if err != nil {
		log.Printf("Failed to store classes: %s", err)
	}
}

func main() {
	var err error
	config, err = gym.NewConfig()
	if err != nil {
		log.Fatal("Unable to get gym configuration")
	}
	// Get and save all classes
	getAndStoreClasses(config)
	us, err := getUsersWithoutClasses(config)
	if err != nil {
		log.Printf("Failed to get any users without classes: %s\n", err)
	} else {
		err := sendEmail(us)
		if err != nil {

			log.Printf("Failed to send email to users: %s\n", err)
		}
	}
	// Set up cron jobs
	c := cron.New()
	c.AddFunc("0 0 13 * * *", func() { fmt.Println("at 1pm") })
	c.AddFunc("0 0 5 * * *", func() { getAndStoreClasses(config) })
	c.Start()

	// Create a new router for requests
	router := mux.NewRouter().StrictSlash(false)

	router.Handle("/classsearch/", Search)
	router.Handle("/classes/", jwtMiddleware.Handler(Classes))
	router.Handle("/preferences/", jwtMiddleware.Handler(Preferences))
	router.Handle("/preferredclasses/", jwtMiddleware.Handler(PreferredClasses))
	router.Handle("/stats/", jwtMiddleware.Handler(Statistics))
	router.Handle("/users/", jwtMiddleware.Handler(Users))
	router.Handle("/slack/", Slack)

	//router.Handle("/", http.FileServer(http.Dir("./ui/")))
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./ui/")))
	// Start the http server
	log.Fatal(http.ListenAndServe(":9000", router))

}

var notFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./ui/index.html")
	//http.FileServer(http.Dir("./ui/"))
	return
})

// TODO: Fix this it is terrible
var jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
	ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
		return jwt.ParseRSAPublicKeyFromPEM([]byte(`-----BEGIN CERTIFICATE-----
MIIC9jCCAd6gAwIBAgIJVdznazUeMN0iMA0GCSqGSIb3DQEBBQUAMCIxIDAeBgNV
BAMTF3J5YW5rc2NvdHQuYXUuYXV0aDAuY29tMB4XDTE3MDEyMTIzMTkxMloXDTMw
MDkzMDIzMTkxMlowIjEgMB4GA1UEAxMXcnlhbmtzY290dC5hdS5hdXRoMC5jb20w
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC3f0iQ3Vuud/E4PDioswEZ
+BcqlDWVXRIW3Nfod3xBz9OlZLN/RSqVl7B9EByVq/qrKD3Kt5bI+XV4X69CQzPI
unzIMqKk7Dq7aZ/rcz95qf4DTbaU7w3M/DYS2eTtEWfFSA57eXJptJ6DOEkya5Sv
Jssmq7xKmlS4l0P5/jWngmVrqfg3ggmXqQgtfMjtlVXLMR6e3rykT0nWhBQfTcf5
Y7fQ3tvtVbog5TFVzH2hB/CEBSa+W1G6LBXTi//4LWkdMvTeLgcC2kUtu2WyHzEt
aK6nqunKriwcxFQyjbn7avrcnzDNpQVspcbsUnAH5gwePWsbJaD4Kcrm+xMjLmpR
AgMBAAGjLzAtMAwGA1UdEwQFMAMBAf8wHQYDVR0OBBYEFCHT4qEoR7xqHyEG3hTb
OITmeVaEMA0GCSqGSIb3DQEBBQUAA4IBAQCpog51RIOvMO2nUdsvtxeFw/NDCzeg
UIKP9lbwfboPOUwaDMAOJ8vudvzjrX/KctLcgo0yN5ctXBRhqQ+5yfy4uq5IzwLy
OiJbgY11rhY+D/8ufyGIvR0arFM0EdMXi4BKP36a3VOK5qMj0GOH9KIbG0gM96if
l0EqjYuYkSbfKCb9b/5GQkioMD5jJW5yL7YCyPjrChq4X0jSOrwkcmpW8q46mDor
yHKwv75eRY/d+Lbb3ZC9Ex/S3hh5bZex97n0i0HtQuiAPJswTsGw5nRsnAuojpyZ
A4/HuY66mTvvFPnGV/Q1YQaQ7pvwOm3YoqIzxBbv0MaOhS14rJGN1m8w
-----END CERTIFICATE-----
`))
	},
	SigningMethod: jwt.SigningMethodRS256,
})

// PreferredClasses returns the preferred classes for a user
var PreferredClasses = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}

	switch r.Method {

	case "OPTIONS":
		w.WriteHeader(200)
		return
	case "GET":
		user := r.Context().Value("user")
		token := user.(*jwt.Token)

		var userID string
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID = claims["sub"].(string)
		} else {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user from JWT token")
			return
		}
		foundPreferences, err := gym.QueryUserPreferences(userID, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user preferences")
			return
		}

		foundClasses, err := gym.QueryPreferredClasses(foundPreferences, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get preferred classes for user")
			return
		}
		classes, err := json.Marshal(foundClasses)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Unable to parse user preferred classes: %s", err)
		}
		w.WriteHeader(200)
		fmt.Fprintf(w, string(classes))
		return
	}
})

// Statistics returns the statistics about a user
var Statistics = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}

	switch r.Method {

	case "OPTIONS":
		w.WriteHeader(200)
		return
	case "GET":
		user := r.Context().Value("user")
		token := user.(*jwt.Token)

		var userID string
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID = claims["sub"].(string)
		} else {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user from JWT token")
			return
		}
		foundStatistics, err := gym.QueryUserStatistics(userID, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user stats")
			return
		}
		stats, err := json.Marshal(foundStatistics)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Unable to parse user stats: %s", err)
			return
		}
		w.WriteHeader(200)
		fmt.Fprintf(w, string(stats))
		return
	}

})

// Preferences returns the preferences a user has for particular classes
var Preferences = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}

	switch r.Method {

	case "OPTIONS":
		w.WriteHeader(200)
		return
	case "GET":
		user := r.Context().Value("user")
		token := user.(*jwt.Token)

		var userID string
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID = claims["sub"].(string)
		} else {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user from JWT token")
			return
		}
		foundPreferences, err := gym.QueryUserPreferences(userID, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user preferences")
			return
		}
		preferences, err := json.Marshal(foundPreferences)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Unable to parse user preferences: %s", err)
		}
		w.WriteHeader(200)
		fmt.Fprintf(w, string(preferences))
		return
	}

})

// Search returns the classes based on a parsed query in natural english
var Search = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}

	var params = r.URL.Query()
	query := params.Get("q")
	if query == "" {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Unable to get search query")
		return
	}
	q, err := gym.QueryClassesByName(query, config)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
		return
	}
	c, err := gym.QueryClasses(q, config)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
		return
	}
	classes, err := json.Marshal(c)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error")
		log.Printf("Unable to parse gymclasses: %s", err)
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, string(classes))
	return

})

// Classes returns the classes a user has saved from the data store
var Classes = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}

	switch r.Method {

	case "GET":
		user := r.Context().Value("user")
		token := user.(*jwt.Token)

		var userID string
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID = claims["sub"].(string)
		} else {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user from JWT token")
			return
		}
		foundClasses, err := gym.QueryUserClasses(userID, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Could not get classes for user")
			log.Printf("Failed to store user class %s", err)
			return
		}
		classes, err := json.Marshal(foundClasses)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Unable to parse user classes: %s", err)
		}
		w.WriteHeader(200)
		fmt.Fprintf(w, string(classes))
		return

	case "POST":
		user := r.Context().Value("user")
		token := user.(*jwt.Token)

		var userID string
		// Here we get the user to do stuff with
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID = claims["sub"].(string)
		} else {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user from JWT token")
			return
		}
		var params = r.URL.Query()
		classID := params.Get("classID")
		if classID == "" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Unable to get class ID")
			return
		}

		err := gym.StoreUserClass(userID, classID, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Could not store class")
			log.Printf("Failed to store user class %s", err)
			return
		}

	case "DELETE":
		user := r.Context().Value("user")
		token := user.(*jwt.Token)

		var userID string
		// Here we get the user to do stuff with
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID = claims["sub"].(string)
		} else {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to get user from JWT token")
			return
		}
		var params = r.URL.Query()
		classID := params.Get("classID")
		if classID == "" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Unable to get class ID")
			return
		}
		log.Printf("Deleting class: %s for user: %s \n", classID, userID)
		err := gym.DeleteUserClass(userID, classID, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Could not delete class")
			log.Printf("Failed to delete user class %s", err)
			return
		}
	}

})

// Slack receives a request and forwards it to a Slack webhook URL
var Slack = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}
	switch r.Method {

	case "OPTIONS":
		w.WriteHeader(200)
		return
	case "POST":
		// TODO: put this in a configuration
		url := "https://hooks.slack.com/services/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"
		req, err := http.NewRequest("POST", url, r.Body)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(w, "Could not post message to Slack")
			log.Printf("Failed to post message to Slack - %s", err)
		}
		// TODO: Check the response code
		defer resp.Body.Close()
		return
	default:
		w.WriteHeader(405)
		return
	}
})

// Users returns the statistics about a user
var Users = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, authorization")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	}

	switch r.Method {

	case "OPTIONS":
		w.WriteHeader(200)
		return
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to read body from request")
			return
		}
		var u gym.User
		err = json.Unmarshal(body, &u)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error")
			log.Printf("Failed to unmarshall body into user")
			return
		}
		err = gym.StoreUser(u, config)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Could not store user")
			log.Printf("Failed to store user %s", err)
			return
		}

		w.WriteHeader(200)
		return
	}

})
