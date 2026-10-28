package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BusinessProfile holds the editable fields of a WhatsApp Business profile.
// Mirrors the Graph API representation.
type BusinessProfile struct {
	About             string   `json:"about,omitempty"`
	Address           string   `json:"address,omitempty"`
	Description       string   `json:"description,omitempty"`
	Email             string   `json:"email,omitempty"`
	ProfilePictureURL string   `json:"profile_picture_url,omitempty"`
	Websites          []string `json:"websites,omitempty"`
	Vertical          string   `json:"vertical,omitempty"`
	// HasCachedPicture is true when the API has a server-side GCS copy
	// of the profile picture that the dashboard can purge with DELETE
	// /channels/:id/whatsapp/profile/picture. Not part of the Meta
	// response — populated by the handler from channel config — so it
	// is omitted on the outbound Meta calls (no `omitempty` issue).
	HasCachedPicture bool `json:"has_cached_picture"`
}

// GetBusinessProfile fetches the current WhatsApp Business profile for a phone number.
func GetBusinessProfile(ctx context.Context, phoneNumberID, accessToken string) (*BusinessProfile, error) {
	apiURL := fmt.Sprintf(
		metaGraphBaseURL+"/%s/%s/whatsapp_business_profile?fields=about,address,description,email,profile_picture_url,websites,vertical",
		metaAPIVersion, phoneNumberID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("whatsapp: profile fetch failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		Data []BusinessProfile `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Data) == 0 {
		return &BusinessProfile{}, nil
	}
	return &wrapper.Data[0], nil
}

// UpdateBusinessProfile updates editable fields of the business profile.
// Pass profilePictureHandle (from UploadProfilePictureToMeta) to also update the picture.
func UpdateBusinessProfile(ctx context.Context, phoneNumberID, accessToken string, profile BusinessProfile, profilePictureHandle string) error {
	apiURL := fmt.Sprintf(
		metaGraphBaseURL+"/%s/%s/whatsapp_business_profile",
		metaAPIVersion, phoneNumberID,
	)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
	}
	if profile.About != "" {
		payload["about"] = profile.About
	}
	if profile.Address != "" {
		payload["address"] = profile.Address
	}
	if profile.Description != "" {
		payload["description"] = profile.Description
	}
	if profile.Email != "" {
		payload["email"] = profile.Email
	}
	if len(profile.Websites) > 0 {
		payload["websites"] = profile.Websites
	}
	if profile.Vertical != "" {
		payload["vertical"] = profile.Vertical
	}
	if profilePictureHandle != "" {
		payload["profile_picture_handle"] = profilePictureHandle
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("whatsapp: profile update failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// UploadProfilePictureToMeta uploads an image to Meta's Resumable Upload API
// and returns the handle to be used in UpdateBusinessProfile.
//
// Flow:
//  1. POST /{app_id}/uploads?file_length=N&file_type=... → {id: upload_id}
//  2. POST /{upload_id} with Authorization: OAuth <token>, raw bytes → {h: handle}
func UploadProfilePictureToMeta(ctx context.Context, appID, accessToken string, fileBytes []byte, mimeType string) (string, error) {
	// Step 1: create upload session
	initURL := fmt.Sprintf(
		metaGraphBaseURL+"/%s/%s/uploads?file_length=%d&file_type=%s&access_token=%s",
		metaAPIVersion, appID, len(fileBytes), mimeType, accessToken,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, initURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("whatsapp: upload init failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	var init struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &init); err != nil {
		return "", err
	}
	if init.ID == "" {
		return "", fmt.Errorf("whatsapp: empty upload id in response: %s", string(body))
	}

	// Step 2: upload the bytes
	uploadURL := fmt.Sprintf(metaGraphBaseURL+"/%s", init.ID)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(fileBytes))
	if err != nil {
		return "", err
	}
	req2.Header.Set("Authorization", "OAuth "+accessToken)
	req2.Header.Set("file_offset", "0")

	resp2, err := metaHTTPClient.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode >= 400 {
		return "", fmt.Errorf("whatsapp: upload bytes failed (HTTP %d): %s", resp2.StatusCode, string(body2))
	}
	var up struct {
		H string `json:"h"`
	}
	if err := json.Unmarshal(body2, &up); err != nil {
		return "", err
	}
	if up.H == "" {
		return "", fmt.Errorf("whatsapp: empty handle in upload response: %s", string(body2))
	}
	return up.H, nil
}
