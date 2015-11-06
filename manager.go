package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"net/http"
	"github.com/gorilla/mux"
	"log"
	"strconv"
	"github.com/dstarodubtsev/go_strftime"
	"flag"
)

// --------------------------------------------------------------------------------
// Command line parameters

var port = flag.Int("port", 7777, "Listening port")
var keys_file = flag.String("keys_file", "./accounts.json", "JSON-file with accounts")

// --------------------------------------------------------------------------------
// Global vars

var default_bullets int64 = 1
var keys_hash = make(map[string]bool)
var services = []string{"search/tweets", "statuses/user_timeline", "statuses/lookup", "users/lookup", "followers/ids", "friends/ids"}
var limits = make(map[string]map[string]Limits)

// --------------------------------------------------------------------------------
// Keys storage

type Key struct {
	ConsumerKey       string `json:"consumer_key"`
	ConsumerSecret    string `json:"consumer_secret"`
	AccessToken       string `json:"access_token"`
	AccessTokenSecret string `json:"access_token_secret"`
}

func(key *Key) Hash() string{
	return fmt.Sprintf("%s::%s::%s::%s", key.ConsumerKey, key.ConsumerSecret, key.AccessToken, key.AccessTokenSecret)
}

func(key Key) String() string{
	return fmt.Sprintf("consumer_key: %s, consumer_secret: %s, access_token: %s, access_token_secret: %s", key.ConsumerKey, key.ConsumerSecret, key.AccessToken, key.AccessTokenSecret)
}

type KeyList []Key

// --------------------------------------------------------------------------------
// Limits pack

type Limits struct {
	Remaining int64
	Reset int64
	Key Key
}

// --------------------------------------------------------------------------------
// Better then if err != nil{...} :)
func check_err(e error) {
	if e != nil {
		log.Panic(e)
	}
}

// --------------------------------------------------------------------------------
// Loading keys from file

func LoadKeys(keys_file string) KeyList {

	// Get absolute path to keys_file
	fp, err := filepath.Abs(keys_file)
	check_err(err)

	// Read file with keys
	f, err := os.Open(fp)
	check_err(err)

	// Decode JSON
	keys_pattern := KeyList{}
	j := json.NewDecoder(f)
	err = j.Decode(&keys_pattern)
	check_err(err)

	// Fill keys hash map
	for _, key := range keys_pattern {
		keys_hash[key.Hash()] = true
	}

	log.Print("Keys loaded")
	return keys_pattern
}

// --------------------------------------------------------------------------------
// Serving requests: get key

func GetKey(w http.ResponseWriter, r *http.Request){
	vars := mux.Vars(r)
	service := vars["service"]

	for k, v := range limits{
		if k != service{
			continue
		}
		for ck, l := range v{
			// Have more then 0 attempts or refresh time is reached
			if l.Remaining > 0 || time.Now().UTC().Unix() > l.Reset {

				// Send as JSON
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(l.Key)
				check_err(err)

				// Delete from storage
				delete(limits[service], ck)

				refresh_str := go_strftime.Strftime("%Y-%m-%d %H:%M:%S UTC", time.Unix(l.Reset, 0).UTC())
				log.Printf("GET key \"%s\" for service %s, remaining: %d, reset: %s, bullets: %d", ck, service, l.Remaining, refresh_str, len(limits[service]))
				return
			}
		}
	}

	// No one suitable key
	w.WriteHeader(http.StatusNoContent)
}

// --------------------------------------------------------------------------------
// Serving requests: set key

func SetKey(w http.ResponseWriter, r *http.Request){
	vars := mux.Vars(r)
	service := vars["service"]

	// Pass only our keys
	for _, s := range services{
		if s == service{
			break
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Read data from requested form
	consumer_key := r.FormValue("consumer_key")
	consumer_secret := r.FormValue("consumer_secret")
	access_token := r.FormValue("access_token")
	access_token_secret := r.FormValue("access_token_secret")

	// Make key
	key := Key{
		ConsumerKey: consumer_key, ConsumerSecret: consumer_secret,
		AccessToken: access_token, AccessTokenSecret: access_token_secret,
	}

	// Check key
	if _, ok := keys_hash[key.Hash()]; !ok{
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Have no key inside keys dictionary")
		return
	}

	// Read optional field "remaining"
	remaining, err := strconv.ParseInt(r.FormValue("remaining"), 10, 64)
	if err != nil{
		remaining = 1
	}

	// Read optional field "reset"
	reset, err := strconv.ParseInt(r.FormValue("reset"), 10, 64)
	if err != nil{
		reset = time.Now().UTC().Unix()
	}

	// App key to storage
	limits[service][key.ConsumerKey] = Limits{Remaining: remaining, Reset: reset, Key: key}

	refresh_str := go_strftime.Strftime("%Y-%m-%d %H:%M:%S UTC", time.Unix(reset, 0).UTC())
	log.Printf("SET key \"%s\" for service \"%s\", remaining: %d, reset: %s, bullets: %d", key.ConsumerKey, service, remaining, refresh_str, len(limits[service]))

	w.WriteHeader(http.StatusOK)
}

func main() {

	flag.Usage = func() {
		fmt.Println(`==========================================================================================
Twitter REST API keys provider. Visit https://apps.twitter.com for details.
Make JSON file with accounts, for example /path/to/accounts.json:
	[
		{
			"consumer_key": "your-consumer-key",
			"consumer_secret" : "your-consumer-secret",
			"access_token": "your-access-token",
			"access_token_secret": "your-access-secret"
		},
		{
			... another key ...
		}
	]

Start application with flags "port" and "keys_file", for example:
	./go_keys_manager --port 7777 --keys_file /path/to/accounts.json

Get key example:
	GET http://localhost:7777/get?service=search/tweets

Set key example:
	POST http://localhost:7777/set?service=search/tweets
	Form fields:
	- consumer_key string (requred)
	- consumer_secret string (requred)
	- access_token string (requred)
	- access_token_secret string (requred)
	- remaining int (current rate limit, optional but useful for next usage)
	- reset int (next rate limit reset UNIX time, optional but useful for next usage)
`)
		// flag.PrintDefaults()
	}
	flag.Parse()

	keys := LoadKeys(*keys_file)

	// Fill service/keys pairs
	for _, service := range services{
		limits[service] = make(map[string]Limits)
		for _, k := range keys {
			limits[service][k.ConsumerKey] = Limits{default_bullets, time.Now().UTC().Unix(), k}
		}
	}

	// Start listening
	router := mux.NewRouter().StrictSlash(true)
	router.Methods("GET").Path("/get").Name("GetKey").Queries("service", "{service:[a-z/]+}").HandlerFunc(GetKey)
	router.Methods("POST").Path("/set").Name("SetKey").Queries("service", "{service:[a-z/]+}").HandlerFunc(SetKey)
	log.Printf("Start listening port %d\n", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), router))

}
