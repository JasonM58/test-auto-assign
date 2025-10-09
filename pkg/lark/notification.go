package lark

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type CardPayload struct {
	MsgType string      `json:"msg_type"`
	Card    interface{} `json:"card"`
}

func SendLarkNotification(secret, webhookURL, message string) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Step 1: Create HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	sign,err := GenSign(secret,time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to marshal lark payload: %w", err)
	}

	// Step 2: Prepare request body
	payload := map[string]interface{}{
		"timestamp": timestamp,
		"sign":      sign,
		"msg_type":  "text",
		"content": map[string]string{
			"text": message,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal lark payload: %w", err)
	}

	// Step 3: Send POST to Lark
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send Lark request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Lark webhook returned non-200 status: %s", resp.Status)
	}

	return nil
}

func GenSign(secret string, timestamp int64) (string, error) {
   stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret

   var data []byte
   h := hmac.New(sha256.New, []byte(stringToSign))
   _, err := h. Write(data)
   if err != nil {
      return "", err
   }

   signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
   return signature, nil
}


func SendLarkCard(secret, webhookURL string, card interface{}) error {
	timestamp := time.Now().Unix()

	sign, err := GenSign(secret, timestamp)
	if err != nil {
		return fmt.Errorf("failed to generate sign: %w", err)
	}

	// Build card payload
	payload := map[string]interface{}{
		"timestamp": fmt.Sprintf("%d", timestamp),
		"sign":      sign,
		"msg_type":  "interactive",
		"card":      card,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal lark payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send Lark request: %w", err)
	}
	defer resp.Body.Close()

	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)
	fmt.Println("Lark response:", resp.StatusCode, respBody.String())

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Lark webhook returned non-200 status: %s", resp.Status)
	}

	return nil
}


