package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

type Configuration struct {
	Domain      string
	Credentials struct {
		Username string
		Password string
	}
}

/* Get our public IP address record by fetching `https://domains.google.com/checkip` */
func getCurrentIP() (ip net.IP) {
	/* Get ipv4 */
	response, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Fatal(err)
	}

	/* Make sure we check closing errors */
	defer func(response *http.Response) {
		if err := response.Body.Close(); err != nil {
			log.Fatal(err)
		}
	}(response)

	ipBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	err = ip.UnmarshalText(ipBytes)
	if err != nil {
		log.Fatal(err)
	}

	return
}

/* Check if we need to update the record */
func needsUpdate(domain string, ip net.IP) (update bool) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		log.Fatal(err)
	}

	for _, currIp := range ips {
		if ip.Equal(currIp.To4()) {
			return
		}
	}
	update = true
	return
}

/* Read configuration file */
func readConfig(filename string) (configuration Configuration) {
	/* Read the whole file into a buffer */
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	/* Parse and validate JSON */
	if err := json.Unmarshal(content, &configuration); err != nil {
		log.Fatal(err)
	} else if configuration.Domain == "" {
		log.Fatal("Missing or empty 'Domain' field in JSON configuration.")
	} else if configuration.Credentials.Username == "" {
		log.Fatal("Missing or empty 'Credentials.Username' field in JSON configuration.")
	} else if configuration.Credentials.Password == "" {
		log.Fatal("Missing or empty 'Credentials.Password' field in JSON configuration.")
	}

	return
}

/* This updates the record for the given IP address using the given configuration */
func updateRecord(configuration Configuration, ip net.IP) {
	url := fmt.Sprintf("https://domains.google.com/nic/update?hostname=%v&myip=%v", configuration.Domain, ip)
	log.Println("Query URL for update:", url)

	client := &http.Client{
		CheckRedirect: nil,
		Timeout:       time.Second * 30,
	}

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	request.SetBasicAuth(configuration.Credentials.Username, configuration.Credentials.Password)

	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}

	/* Make sure we check closing errors */
	defer func(response *http.Response) {
		if err := response.Body.Close(); err != nil {
			log.Fatal(err)
		}
	}(response)

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Response status:", response.StatusCode, response.Status, "Response:", string(body))
}

/* Entrypoint of our program */
func main() {
	filename := "examples/configuration.json"
	log.Println("Using configuration file:", filename)

	configuration := readConfig(filename)
	log.Println("Found valid configuration for domain:", configuration.Domain)

	ipv4 := getCurrentIP()
	log.Println("Current IPv4:", ipv4)

	if needsUpdate(configuration.Domain, ipv4) {
		log.Println("Updating record")
		updateRecord(configuration, ipv4)
	} else {
		log.Println("No need to update record")
	}
}