package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

type configuration struct {
	SleepTime int64
	Domain      string
	Credentials struct {
		Username string
		Password string
	}
}

/* Read configuration file */
func readConfig(filename string) (configuration configuration) {
	/* Read the whole file into a buffer */
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	/* Parse and validate JSON */
	if err := json.Unmarshal(content, &configuration); err != nil {
		log.Fatal(err)
	} else if configuration.Domain == "" {
		log.Println("Missing or empty 'Domain' field in JSON configuration.")
	} else if configuration.Credentials.Username == "" {
		log.Println("Missing or empty 'Credentials.Username' field in JSON configuration.")
	} else if configuration.Credentials.Password == "" {
		log.Println("Missing or empty 'Credentials.Password' field in JSON configuration.")
	} else if configuration.SleepTime <= 0 {
		configuration.SleepTime = 1
	}

	return
}

/* Get our public IP address record by fetching `https://domains.google.com/checkip` */
func getCurrentIP() (ip net.IP, err error) {
	/* Get ipv4 */
	response, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Println(err)
		return
	}

	/* Make sure we check closing errors */
	defer func(response *http.Response) {
		if err := response.Body.Close(); err != nil {
			log.Println(err)
		}
	}(response)

	ipBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
		return
	}

	err = ip.UnmarshalText(ipBytes)
	if err != nil {
		log.Println(err)
		return
	}
	return
}

/* Check if we need to update the record */
func needsUpdate(domain string, ip net.IP) (update bool, err error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		log.Println(err)
		return
	}

	for _, currIp := range ips {
		if ip.Equal(currIp.To4()) {
			return
		}
	}
	update = true
	return
}

/* This updates the record for the given IP address using the given configuration */
func updateRecord(configuration configuration, ip net.IP) (err error) {
	url := fmt.Sprintf("https://domains.google.com/nic/update?hostname=%v&myip=%v", configuration.Domain, ip)
	log.Println("Query URL for update:", url)

	client := &http.Client{
		CheckRedirect: nil,
		Timeout:       time.Second * 30,
	}

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Println(err)
		return
	}
	request.SetBasicAuth(configuration.Credentials.Username, configuration.Credentials.Password)

	response, err := client.Do(request)
	if err != nil {
		log.Println(err)
		return
	}

	/* Make sure we check closing errors */
	defer func(response *http.Response) {
		if err := response.Body.Close(); err != nil {
			log.Println(err)
		}
	}(response)

	body, err := ioutil.ReadAll(response.Body)
	strBody := string(body)

	if err != nil {
		log.Println(err)
		return
	} else if strBody != fmt.Sprintf("good %v", ip) && strBody != fmt.Sprintf("nochg %v", ip) {
		log.Println(" Response: ", strBody)
	}

	log.Println("Response:", strBody)
	return
}
/* Do the actual magic */
func updateHandler(configuration configuration) {
	ipv4, err := getCurrentIP()
	if err != nil {
		log.Println("Could not get public IP waiting for next run")
		return
	}
	log.Println("Current IPv4:", ipv4)

	log.Println("Checking if we need to update our record")
	needUpdate, err := needsUpdate(configuration.Domain, ipv4)
	if err != nil {
		log.Println("Couldn't check for update")
		return
	} else if needUpdate {
		log.Println("Updating record")
		if err := updateRecord(configuration, ipv4); err != nil {
			log.Println("Failed to update our IP")
			return
		}
	} else {
		log.Println("No need to update record")
	}
}

/* Entrypoint of our program */
func main() {
	filename := flag.String("config", "/etc/zorua/config.json", "path to the JSON configuration file")
	flag.Parse()

	log.Println("Using configuration file:", *filename)
	configuration := readConfig(*filename)
	log.Println("Found valid configuration for domain:", configuration.Domain)
	log.Println("Sleep time is", configuration.SleepTime, "minute(s)")

	updateHandler(configuration)
	log.Println("Sleeping for", configuration.SleepTime, "minute(s)")

	for _ = range time.NewTicker(time.Duration(configuration.SleepTime) * time.Minute).C {
		log.Println("Waking up")
		updateHandler(configuration)
		log.Println("Sleeping for", configuration.SleepTime, "minute(s)")
	}
}
