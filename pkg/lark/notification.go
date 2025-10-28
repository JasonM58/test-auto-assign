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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Lark webhook returned non-200 status: %s", resp.Status)
	}

	return nil
}

func SendLarkWebhookMessage(webhookURL, secret string, msg map[string]interface{}) error {
	payload := msg

	if secret != "" {
		timestamp := time.Now().Unix()
		sign, err := GenSign(secret, timestamp)
		if err != nil {
			return fmt.Errorf("failed to sign Lark message: %w", err)
		}
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = sign
	}

	body, _ := json.Marshal(payload)
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
