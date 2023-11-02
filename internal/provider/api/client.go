package uptimerobotapi

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

func New(apiKey string, cacheTtl int) UptimeRobotApiClient {
	return UptimeRobotApiClient{apiKey, cacheTtl, &sync.Mutex{}}
}

type UptimeRobotApiClient struct {
	apiKey   string
	cacheTtl int
	mutex    *sync.Mutex
}

func WaitOnRateLimit(l retryablehttp.Logger, res *http.Response) {
	if res == nil {
		return
	}

	retryAfterStr := res.Header.Get("retry-after")
	retryAfter, err := strconv.Atoi(retryAfterStr)
	if err != nil {
		log.Printf("[ERROR] Error parsing Retry-After header %s: %e", retryAfterStr, err)
	}

	log.Printf("[DEBUG] Got rate limit, sleep %d seconds", retryAfter)
	time.Sleep(time.Second * time.Duration(retryAfter))
}

func (client UptimeRobotApiClient) makeCall(
	endpoint string,
	params string,
) ([]byte, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	log.Printf("[DEBUG] Making request to: %#v", endpoint)

	url := "https://api.uptimerobot.com/v2/" + endpoint

	payload := strings.NewReader(
		fmt.Sprintf("api_key=%s&format=json&%s", client.apiKey, params),
	)

	req, _ := http.NewRequest("POST", url, payload)

	req.Header.Add("cache-control", "no-cache")
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10
	retryClient.ResponseLogHook = WaitOnRateLimit
	standardClient := retryClient.StandardClient()

	res, err := standardClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Got %d response from UptimeRobot: %s", res.StatusCode, body)
	}

	log.Printf("[DEBUG] Got response: %#v", res)
	log.Printf("[DEBUG] Got body: %#v", string(body))

	return body, nil
}

func (client UptimeRobotApiClient) decodeCall(body []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(body), &result)
	if err != nil {
		return nil, fmt.Errorf("Got decoding json from UptimeRobot: %s. Response body: %s", err.Error(), body)
	}

	if result["stat"] != "ok" {
		message, _ := json.Marshal(result["error"])
		return nil, errors.New("Got error from UptimeRobot: " + string(message))
	}

	return result, nil
}

func (client UptimeRobotApiClient) MakeCall(
	endpoint string,
	params string,
) (map[string]interface{}, error) {
	body, err := client.makeCall(endpoint, params)
	if err != nil {
		return nil, err
	}

	result, err := client.decodeCall(body)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (client UptimeRobotApiClient) getCachePath(
	endpoint string,
	params string,
) (string, error) {
	// Calculate request hash
	hasher := sha512.New()
	hasher.Write([]byte(endpoint))
	hasher.Write([]byte(params))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Calculate cache path
	homeDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(homeDir, "terraform-uptimerobot")
	err = os.MkdirAll(cacheDir, 0750)
	if err != nil {
		return "", err
	}

	cachePath := filepath.Join(cacheDir, hash)
	return cachePath, nil
}

func (client UptimeRobotApiClient) readCache(cachePath string) []byte {
	var body []byte
	stats, err := os.Stat(cachePath)
	if err != nil {
		return nil
	}

	if stats.ModTime().After(time.Now().Add(time.Duration(-client.cacheTtl) * time.Second)) {
		body, err = os.ReadFile(cachePath)
		if err != nil {
			return nil
		} else {
			log.Printf("[DEBUG] Cache hit: %s", cachePath)
			return body
		}
	}

	return nil
}

func (client UptimeRobotApiClient) MakeCallCachable(
	endpoint string,
	params string,
) (map[string]interface{}, error) {
	var body []byte = nil
	cachePath, err := client.getCachePath(endpoint, params)
	if err == nil {
		body = client.readCache(cachePath)
	}

	if body == nil {
		body, err = client.makeCall(endpoint, params)
	}

	if err != nil {
		return nil, err
	}

	result, err := client.decodeCall(body)
	if err != nil {
		return nil, err
	}

	// Ignore error on writing cache
	os.WriteFile(cachePath, body, 0640)

	return result, nil
}
