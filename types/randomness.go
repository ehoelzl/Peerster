package types

import (
	"log"
	"os"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"errors"
)



type ETHINFO struct {
	Name             string    `json:"name"`
	Height           int       `json:"height"`
	Hash             string    `json:"hash"`
	LatestURL        string    `json:"latest_url"`
	PreviousHash     string    `json:"previous_hash"`
	PreviousURL      string    `json:"previous_url"`
	PeerCount        int       `json:"peer_count"`
	UnconfirmedCount int       `json:"unconfirmed_count"`
	HighGasPrice     int64     `json:"high_gas_price"`
	MediumGasPrice   int64     `json:"medium_gas_price"`
	LowGasPrice      int64     `json:"low_gas_price"`
	LastForkHeight   int       `json:"last_fork_height"`
	LastForkHash     string    `json:"last_fork_hash"`
}

type Randomness struct {
	Round float64
	Point string
}

// getRandomness fetches the latest randomness value from the Distributed
// Randomness API.
//
// Example response from the API :
// ```
// {
//     "round":289541,
//     "previous":"7bec00e0cdf950bd42883143d9222ee2a327cc72542e66f6c66ac2f238273efd129d29816a7542b288f1ec8738b938cb7955c22c966fb13c6b90a86324afb526",
//     "randomness":{
//         "gid":21,
//         "point":"35ebfe71b0a188f29779475753a77303bc605fb6a9ac848e65c4a36a56ae154437b578436d4cf000f41cd32e7fa819f7c0b8cdfc4c66c1a6750767f875f8aae4"
//     }
// }
// ```
func getRandomness() (*Randomness, error) {
	os.Setenv("GODEBUG", "tls13=0")
	resp, err := http.Get("https://drand.cloudflare.com/api/public")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var value map[string]interface{}
	json.Unmarshal(body, &value)
	round, ok := value["round"].(float64)
	if !ok {
		return nil, errors.New("Could not get round from cloudflare response")
	}
	randomnessMap, ok := value["randomness"].(map[string]interface{})
	if !ok {
		return nil, errors.New("Could not get randomness object from cloudflare response")
	}
	point, ok := randomnessMap["point"].(string)
	if !ok {
		return nil, errors.New("Could not get point from cloudflare response")
	}
	randomness := Randomness{Round: round, Point: point}
	return &randomness, nil
}



func getRandomnessETH()(hash string){
	resp, err := http.Get("https://api.blockcypher.com/v1/eth/main")
	if err != nil {
		log.Println("Error with the blockcypher api, use another source of randomness")
	}
	defer resp.Body.Close()
	eth := ETHINFO{}
	json.NewDecoder(resp.Body).Decode(&eth)
	return eth.Hash
}



