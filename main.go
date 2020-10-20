package main

import (
	"flag"
	"fmt"
	"github.com/GoMetric/go-statsd-client"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Version is a current git commit hash and tag
// Injected by compilation flag
var Version = "Unknown"

// BuildNumber is a current commit hash
// Injected by compilation flag
var BuildNumber = "Unknown"

// BuildDate is a date of build
// Injected by compilation flag
var BuildDate = "Unknown"

// HTTP connection params
const defaultHTTPHost = "127.0.0.1"
const defaultHTTPPort = 80

// StatsD connection params
const defaultStatsDHost = "127.0.0.1"
const defaultStatsDPort = 8125

// JWT params
const jwtHeaderName = "X-JWT-Token"
const jwtQueryStringKeyName = "token"

// declare command line options
var httpHost = flag.String("http-host", defaultHTTPHost, "HTTP Host")
var httpPort = flag.Int("http-port", defaultHTTPPort, "HTTP Port")
var tlsCert = flag.String("tls-cert", "", "TLS certificate to enable HTTPS")
var tlsKey = flag.String("tls-key", "", "TLS private key  to enable HTTPS")
var statsdHost = flag.String("statsd-host", defaultStatsDHost, "StatsD Host")
var statsdPort = flag.Int("statsd-port", defaultStatsDPort, "StatsD Port")
var metricPrefix = flag.String("metric-prefix", "", "Prefix of metric name")
var tokenSecret = flag.String("jwt-secret", "", "Secret to encrypt JWT")
var verbose = flag.Bool("verbose", false, "Verbose")
var version = flag.Bool("version", false, "Show version")

// statsd client
var statsdClient *statsd.Client

func main() {
	// get flags
	flag.Parse()

	// show version and exit
	if *version == true {
		showVersion()
		os.Exit(0)
	}

	// configure verbosity of logging
	if *verbose == true {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	// prepare metric prefix
	if *metricPrefix != "" && (*metricPrefix)[len(*metricPrefix)-1:] != "." {
		*metricPrefix = *metricPrefix + "."
	}

	// create HTTP router
	router := mux.NewRouter().StrictSlash(true)

	// register http request handlers
	router.Handle(
		"/heartbeat",
		validateCORS(http.HandlerFunc(handleHeartbeatRequest)),
	).Methods("GET")

	router.Handle(
		"/count/{key}",
		validateCORS(validateJWT(http.HandlerFunc(handleCountRequest))),
	).Methods("POST")

	router.Handle(
		"/gauge/{key}",
		validateCORS(validateJWT(http.HandlerFunc(handleGaugeRequest))),
	).Methods("POST")

	router.Handle(
		"/timing/{key}",
		validateCORS(validateJWT(http.HandlerFunc(handleTimingRequest))),
	).Methods("POST")

	router.Handle(
		"/set/{key}",
		validateCORS(validateJWT(http.HandlerFunc(handleSetRequest))),
	).Methods("POST")

	router.PathPrefix("/").Methods("OPTIONS").HandlerFunc(handlePreFlightCORSRequest)

	// Create a new StatsD connection
	statsdClient = statsd.NewClient(*statsdHost, *statsdPort)
	statsdClient.Open()
	defer statsdClient.Close()

	// get server address to bind
	httpAddress := fmt.Sprintf("%s:%d", *httpHost, *httpPort)
	log.Printf("Starting HTTP server at %s", httpAddress)

	if *tokenSecret != "" {
		log.Printf("Starting with token")
	} else {
		log.Printf("Starting without token")
	}


	// create http server
	s := &http.Server{
		Addr:           httpAddress,
		Handler:        router,
		ReadTimeout:    1 * time.Second,
		WriteTimeout:   1 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	var err error
	if len(*tlsCert) > 0 && len(*tlsKey) > 0 {
		// start https server
		err = s.ListenAndServeTLS(*tlsCert, *tlsKey)
	} else {
		// start http server
		err = s.ListenAndServe()
	}

	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

// validate CORS headers
func validateCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Access-Control-Allow-Origin", origin)
		}
		next.ServeHTTP(w, r)
	})
}

// validate JWT middleware
func validateJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *tokenSecret == "" {
			next.ServeHTTP(w, r)
		} else {
			// get JWT from header
			tokenString := r.Header.Get(jwtHeaderName)

			log.Printf("Token secret: %s", *tokenSecret)
			log.Printf("Token string: %s", tokenString)

			// get JWT from query string
			if tokenString == "" {
				tokenString = r.URL.Query().Get(jwtQueryStringKeyName)
			}

			if tokenString == "" {
				http.Error(w, "Token not specified", 401)
				return
			}

			// parse JWT
			_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(*tokenSecret), nil
			})

			if err != nil {
				http.Error(w, "Error parsing token", 403)
				return
			}

			// accept request
			next.ServeHTTP(w, r)
		}

	})
}

// Handle heartbeat request
func handleHeartbeatRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "OK")
}

// Handle StatsD Count request
func handleCountRequest(w http.ResponseWriter, r *http.Request) {
	// get key
	vars := mux.Vars(r)
	key := *metricPrefix + vars["key"]

	// get count value
	var value = 1
	valuePostFormValue := r.PostFormValue("value")
	if valuePostFormValue != "" {
		var err error
		value, err = strconv.Atoi(valuePostFormValue)
		if err != nil {
			http.Error(w, "Invalid value specified", 400)
		}
	}

	// get sample rate
	var sampleRate float64 = 1
	sampleRatePostFormValue := r.PostFormValue("sampleRate")
	if sampleRatePostFormValue != "" {
		var err error
		sampleRate, err = strconv.ParseFloat(sampleRatePostFormValue, 32)
		if err != nil {
			http.Error(w, "Invalid sample rate specified", 400)
		}

	}

	// send request
	statsdClient.Count(key, value, float32(sampleRate))
}

// Handle StatsD Gauge request
func handleGaugeRequest(w http.ResponseWriter, r *http.Request) {
	// get key
	vars := mux.Vars(r)
	key := *metricPrefix + vars["key"]

	// get gauge shift
	shiftPostFormValue := r.PostFormValue("shift")
	if shiftPostFormValue != "" {
		// get value
		value, err := strconv.Atoi(shiftPostFormValue)
		if err != nil {
			http.Error(w, "Invalid gauge shift specified", 400)
		}
		// send request
		statsdClient.GaugeShift(key, value)
		return
	}

	// get gauge value
	var value = 1
	valuePostFormValue := r.PostFormValue("value")
	if valuePostFormValue != "" {
		// get value
		var err error
		value, err = strconv.Atoi(valuePostFormValue)
		if err != nil {
			http.Error(w, "Invalid gauge value specified", 400)
		}
	}

	// send gauge value request
	statsdClient.Gauge(key, value)

}

// Handle StatsD Timing request
func handleTimingRequest(w http.ResponseWriter, r *http.Request) {
	// get key
	vars := mux.Vars(r)
	key := *metricPrefix + vars["key"]

	// get timing
	time, err := strconv.ParseInt(r.PostFormValue("time"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid time specified", 400)
	}

	// get sample rate
	var sampleRate float64 = 1
	sampleRatePostFormValue := r.PostFormValue("sampleRate")
	if sampleRatePostFormValue != "" {
		var err error
		sampleRate, err = strconv.ParseFloat(sampleRatePostFormValue, 32)
		if err != nil {
			http.Error(w, "Invalid sample rate specified", 400)
		}
	}

	log.Printf("key: %s time: %d rate: %s", key, time, sampleRate)
	// send request
	statsdClient.Timing(key, time, float32(sampleRate))
}

// Handle StatsD Set request
func handleSetRequest(w http.ResponseWriter, r *http.Request) {
	// get key
	vars := mux.Vars(r)
	key := *metricPrefix + vars["key"]

	// get set value
	var value = 1
	valuePostFormValue := r.PostFormValue("value")
	if valuePostFormValue != "" {
		var err error
		value, err = strconv.Atoi(valuePostFormValue)
		if err != nil {
			http.Error(w, "Invalid set value specified", 400)
		}
	}

	// send request
	statsdClient.Set(key, value)
}

// Handle PreFlight CORS request with OPTIONS method
func handlePreFlightCORSRequest(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Headers", jwtHeaderName+", X-Requested-With, Origin, Accept, Content-Type, Authentication")
		w.Header().Add("Access-Control-Allow-Methods", "GET, POST, HEAD, OPTIONS")
	}
}

func showVersion() {
	fmt.Printf(
		"StatsD HTTP Proxy v.%s, build %s from %s\n",
		Version,
		BuildNumber,
		BuildDate,
	)
}
