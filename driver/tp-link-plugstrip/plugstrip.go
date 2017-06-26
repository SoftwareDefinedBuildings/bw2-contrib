/*
 * Based on reverse engineering of the TP-Link protocol by softSCheck
 * https://www.softscheck.com/en/reverse-engineering-tp-link-hs110/
 */

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const TP_LINK_PORT = 9999
const TP_LINK_KEY_INIT = 171

type Plugstrip struct {
	address string
	model   string
	state   bool
}

type PowerStats struct {
	Current float64
	Voltage float64
	Power   float64
	Total   float64
	State   int
}

type SystemResponse struct {
	System struct {
		Get_sysinfo struct {
			Relay_state int
		}
	}
}

type GetRealtimeResponse struct {
	Emeter struct {
		Get_realtime struct {
			Current float64
			Voltage float64
			Power   float64
			Total   float64
		}
	}
}

func NewPlugstrip(ip string) (*Plugstrip, error) {
	address := fmt.Sprintf("%s:%d", ip, TP_LINK_PORT)
	ps := Plugstrip{address: address}

	response, err := ps.transact(`{"system":{"get_sysinfo":null}}`)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve plugstrip info: %v", err)
	}
	matches := regexp.MustCompile(`\"model\":\"(.+?)\"`).FindStringSubmatch(response)
	if matches == nil {
		return nil, fmt.Errorf("Plugstrip response did not contain model information")
	}
	ps.model = matches[1]
	return &ps, nil
}

func (ps *Plugstrip) SetRelayState(on bool) error {
	var state int
	if on {
		state = 1
	} else {
		state = 0
	}

	payload := fmt.Sprintf(`{"system":{"set_relay_state":{"state":%d}}}`, state)
	if _, err := ps.transact(payload); err != nil {
		return fmt.Errorf("Failed to set plug relay state: %v", err)
	}
	return nil
}

func (ps *Plugstrip) ClearDelayedAction() error {
	if _, err := ps.transact(`{"count_down":{"delete_all_rules":null}}`); err != nil {
		return fmt.Errorf("Failed to clear count down rules: %v", err)
	}
	return nil
}

func (ps *Plugstrip) SetRelayStateDelay(on bool, delay time.Duration) error {
	var state int
	if on {
		state = 1
	} else {
		state = 0
	}

	if err := ps.ClearDelayedAction(); err != nil {
		return fmt.Errorf("Failed to clear previous action: %v", err)
	}
	payload := fmt.Sprintf(`{"count_down":{"add_rule":{"enable":1,"delay":%d,"act":%d,"name":"actuate"}}}`,
		int(delay.Seconds()), state)
	if _, err := ps.transact(payload); err != nil {
		return fmt.Errorf("Failed to add new countdown actuation rule: %v", err)
	}
	return nil
}

func (ps *Plugstrip) HasPowerStats() bool {
	return strings.HasPrefix(ps.model, "HS110")
}

func (ps *Plugstrip) GetState() bool {
	return ps.state
}

func (ps *Plugstrip) GetPowerStats() (*PowerStats, error) {
	if !strings.HasPrefix(ps.model, "HS110") {
		return nil, fmt.Errorf("Power statistics require HS110 model plug")
	}

	system_response, err := ps.transact(`{"system":{"get_sysinfo":{}}}`)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("Failed to retrieve system response: %v", err)
	}

	var sysresp = &SystemResponse{}
	if err := json.Unmarshal([]byte(system_response), sysresp); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON response: %v", err)
	}
	fmt.Println(sysresp)

	response, err := ps.transact(`{"emeter":{"get_realtime":{}}}`)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve power statistics: %v", err)
	}
	fmt.Println(response)
	var resp = &GetRealtimeResponse{}
	if err := json.Unmarshal([]byte(response), resp); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON response: %v", err)
	}
	fmt.Println(resp)

	return &PowerStats{
		Current: resp.Emeter.Get_realtime.Current,
		Voltage: resp.Emeter.Get_realtime.Voltage,
		Power:   resp.Emeter.Get_realtime.Power,
		Total:   resp.Emeter.Get_realtime.Total,
		State:   sysresp.System.Get_sysinfo.Relay_state,
	}, nil
}

func (ps *Plugstrip) transact(command string) (string, error) {
	conn, err := net.Dial("tcp", ps.address)
	if err != nil {
		return "", fmt.Errorf("Failed to connect to plugstrip: %v", err)
	}
	defer conn.Close()

	cipherText := encryptPayload([]byte(command))
	_, err = conn.Write(cipherText)
	if err != nil {
		return "", fmt.Errorf("Failed to send command to plugstrip: %v", err)
	}

	response, err := ioutil.ReadAll(conn)
	if err != nil {
		return "", fmt.Errorf("Failed to read reply from plugstrip: %v", err)
	}
	plainText := string(decryptPayload(response))
	code, err := extractErrorCode(plainText)
	if err != nil {
		return plainText, err
	} else if code != 0 {
		msg, err := extractErrorMessage(plainText)
		if err != nil {
			return plainText, fmt.Errorf("Plugstrip returned non-zero error code: %d (%s)", code, msg)
		}
		return plainText, fmt.Errorf("Plugstrip returned non-zero error code: %d", code)
	}
	return plainText, nil
}

func extractErrorCode(response string) (int64, error) {
	match := regexp.MustCompile(`\"err_code\":(\d+)`).FindStringSubmatch(response)
	if match == nil {
		return 0, errors.New("Could not find error code in response")
	}
	code, err := strconv.ParseInt(match[1], 0, 64)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse error code: %v", err)
	}
	return code, nil
}

func extractErrorMessage(response string) (string, error) {
	re := regexp.MustCompile(`"err_msg":"(.*?)"`)
	match := re.FindStringSubmatch(response)
	if match == nil {
		return "", errors.New("Could not find error message in response")
	}
	return match[1], nil
}

func encryptPayload(plainText []byte) []byte {
	cipherText := make([]byte, len(plainText)+4)
	key := byte(TP_LINK_KEY_INIT)
	for i := 0; i < len(plainText); i++ {
		cipherText[i+4] = key ^ plainText[i]
		key = cipherText[i+4]
	}
	return cipherText
}

func decryptPayload(cipherText []byte) []byte {
	cipherText = cipherText[4:]
	plainText := make([]byte, len(cipherText))
	key := byte(TP_LINK_KEY_INIT)
	for i := 0; i < len(cipherText); i++ {
		plainText[i] = key ^ cipherText[i]
		key = cipherText[i]
	}
	return plainText
}
