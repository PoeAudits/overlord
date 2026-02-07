package syncthing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client provides access to the Syncthing REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient returns a new Syncthing REST client.
func NewClient(baseURL, apiKey string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Device represents a Syncthing device configuration.
type Device struct {
	DeviceID    string   `json:"deviceID"`
	Name        string   `json:"name,omitempty"`
	Addresses   []string `json:"addresses,omitempty"`
	Compression string   `json:"compression,omitempty"`
}

// FolderDevice represents a device entry within a folder configuration.
type FolderDevice struct {
	DeviceID string `json:"deviceID"`
}

// Folder represents a Syncthing folder configuration.
type Folder struct {
	ID               string         `json:"id"`
	Label            string         `json:"label"`
	Path             string         `json:"path"`
	Type             string         `json:"type"`
	Devices          []FolderDevice `json:"devices"`
	FSWatcherEnabled bool           `json:"fsWatcherEnabled,omitempty"`
	Copiers          int            `json:"copiers,omitempty"`
	Hashers          int            `json:"hashers,omitempty"`
	CaseSensitiveFS  bool           `json:"caseSensitiveFS,omitempty"`
}

// FolderStatus represents a subset of /rest/db/status fields.
type FolderStatus struct {
	State       string `json:"state"`
	NeedBytes   int64  `json:"needBytes"`
	LocalBytes  int64  `json:"localBytes"`
	GlobalBytes int64  `json:"globalBytes"`
	Errors      int    `json:"errors"`
	PullErrors  int    `json:"pullErrors"`
}

// GetDevices returns configured devices.
func (c *Client) GetDevices(ctx context.Context) ([]Device, error) {
	var devices []Device
	if err := c.getJSON(ctx, "/rest/config/devices", &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

// GetDevice returns a single device by ID.
func (c *Client) GetDevice(ctx context.Context, id string) (*Device, error) {
	var device Device
	if err := c.getJSON(ctx, "/rest/config/devices/"+url.PathEscape(id), &device); err != nil {
		return nil, err
	}
	return &device, nil
}

// AddDevice creates a new device entry.
func (c *Client) AddDevice(ctx context.Context, device Device) error {
	return c.postJSON(ctx, "/rest/config/devices", device, nil)
}

// UpdateDevice updates an existing device entry.
func (c *Client) UpdateDevice(ctx context.Context, device Device) error {
	return c.putJSON(ctx, "/rest/config/devices/"+url.PathEscape(device.DeviceID), device, nil)
}

// EnsureDevice creates or updates a device configuration.
func (c *Client) EnsureDevice(ctx context.Context, device Device) (bool, error) {
	current, err := c.GetDevice(ctx, device.DeviceID)
	if err != nil {
		if IsNotFound(err) {
			return true, c.AddDevice(ctx, device)
		}
		return false, err
	}

	if device.Name == "" && device.Compression == "" && len(device.Addresses) == 0 {
		return false, nil
	}

	updated := false
	if device.Name != "" && device.Name != current.Name {
		current.Name = device.Name
		updated = true
	}
	if device.Compression != "" && device.Compression != current.Compression {
		current.Compression = device.Compression
		updated = true
	}
	if len(device.Addresses) > 0 && !stringSliceEqual(device.Addresses, current.Addresses) {
		current.Addresses = device.Addresses
		updated = true
	}

	if !updated {
		return false, nil
	}

	return true, c.UpdateDevice(ctx, *current)
}

// GetFolders returns configured folders.
func (c *Client) GetFolders(ctx context.Context) ([]Folder, error) {
	var folders []Folder
	if err := c.getJSON(ctx, "/rest/config/folders", &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

// GetFolder returns a folder by ID.
func (c *Client) GetFolder(ctx context.Context, id string) (*Folder, error) {
	var folder Folder
	if err := c.getJSON(ctx, "/rest/config/folders/"+url.PathEscape(id), &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// AddFolder creates a new folder entry.
func (c *Client) AddFolder(ctx context.Context, folder Folder) error {
	return c.postJSON(ctx, "/rest/config/folders", folder, nil)
}

// UpdateFolder updates an existing folder entry.
func (c *Client) UpdateFolder(ctx context.Context, folder Folder) error {
	return c.putJSON(ctx, "/rest/config/folders/"+url.PathEscape(folder.ID), folder, nil)
}

// EnsureFolder creates or updates a folder configuration.
func (c *Client) EnsureFolder(ctx context.Context, folder Folder) (bool, error) {
	current, err := c.GetFolder(ctx, folder.ID)
	if err != nil {
		if IsNotFound(err) {
			return true, c.AddFolder(ctx, folder)
		}
		return false, err
	}

	updated := mergeFolder(current, folder)
	if !updated {
		return false, nil
	}

	return true, c.UpdateFolder(ctx, *current)
}

// DeleteFolder removes a folder entry.
func (c *Client) DeleteFolder(ctx context.Context, id string) error {
	return c.delete(ctx, "/rest/config/folders/"+url.PathEscape(id))
}

// GetFolderStatus returns status for a folder.
func (c *Client) GetFolderStatus(ctx context.Context, id string) (*FolderStatus, error) {
	endpoint := "/rest/db/status?folder=" + url.QueryEscape(id)
	var status FolderStatus
	if err := c.getJSON(ctx, endpoint, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) postJSON(ctx context.Context, endpoint string, payload any, out any) error {
	return c.writeJSON(ctx, http.MethodPost, endpoint, payload, out)
}

func (c *Client) putJSON(ctx context.Context, endpoint string, payload any, out any) error {
	return c.writeJSON(ctx, http.MethodPut, endpoint, payload, out)
}

func (c *Client) writeJSON(ctx context.Context, method, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	req, err := c.newRequest(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, out)
}

func (c *Client) delete(ctx context.Context, endpoint string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	fullURL, err := joinURL(c.baseURL, endpoint)
	if err != nil {
		return nil, err
	}
	return http.NewRequestWithContext(ctx, method, fullURL, body)
}

func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return httpError{statusCode: resp.StatusCode, body: strings.TrimSpace(string(body))}
	}

	if out == nil {
		return nil
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

type httpError struct {
	statusCode int
	body       string
}

func (e httpError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("syncthing api error: %d", e.statusCode)
	}
	return fmt.Sprintf("syncthing api error: %d: %s", e.statusCode, e.body)
}

// IsNotFound returns true when the API reports a 404 status.
func IsNotFound(err error) bool {
	var httpErr httpError
	return errors.As(err, &httpErr) && httpErr.statusCode == http.StatusNotFound
}

func joinURL(base, endpoint string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint: %w", err)
	}
	baseURL.Path = path.Join(baseURL.Path, endpointURL.Path)
	baseURL.RawQuery = endpointURL.RawQuery
	return baseURL.String(), nil
}

func mergeFolder(current *Folder, desired Folder) bool {
	updated := false

	if desired.Label != "" && current.Label != desired.Label {
		current.Label = desired.Label
		updated = true
	}
	if desired.Path != "" && current.Path != desired.Path {
		current.Path = desired.Path
		updated = true
	}
	if desired.Type != "" && current.Type != desired.Type {
		current.Type = desired.Type
		updated = true
	}
	if desired.FSWatcherEnabled && current.FSWatcherEnabled != desired.FSWatcherEnabled {
		current.FSWatcherEnabled = desired.FSWatcherEnabled
		updated = true
	}
	if desired.Copiers > 0 && current.Copiers != desired.Copiers {
		current.Copiers = desired.Copiers
		updated = true
	}
	if desired.Hashers > 0 && current.Hashers != desired.Hashers {
		current.Hashers = desired.Hashers
		updated = true
	}
	if desired.CaseSensitiveFS && current.CaseSensitiveFS != desired.CaseSensitiveFS {
		current.CaseSensitiveFS = desired.CaseSensitiveFS
		updated = true
	}
	if len(desired.Devices) > 0 && !folderDevicesEqual(current.Devices, desired.Devices) {
		current.Devices = desired.Devices
		updated = true
	}

	return updated
}

func folderDevicesEqual(a, b []FolderDevice) bool {
	if len(a) != len(b) {
		return false
	}
	lookup := make(map[string]struct{}, len(a))
	for _, dev := range a {
		lookup[dev.DeviceID] = struct{}{}
	}
	for _, dev := range b {
		if _, ok := lookup[dev.DeviceID]; !ok {
			return false
		}
	}
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
