// Credits: https://github.com/aellwein/netcup-dns-api/tree/main

package netcup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	// API endpoint for JSON requests
	netcupApiEndpointJSON = "https://ccp.netcup.net/run/webservice/servers/endpoint.php?JSON"
	// JSON content type
	netcupApiContentType = "application/json"
	// Default request timeout
	defaultRequestTimeout = 30 * time.Second
)

// Type for action field of a request payload
type RequestAction string

// type for status
type ResponseStatus string

const (
	StatusSuccess ResponseStatus = "success"
	StatusError   ResponseStatus = "error"
	StatusStarted ResponseStatus = "started"
	StatusPending ResponseStatus = "pending"
	StatusWarning ResponseStatus = "warning"
)

const (
	actionLogin            RequestAction = "login"
	actionLogout           RequestAction = "logout"
	actionInfoDnsZone      RequestAction = "infoDnsZone"
	actionInfoDnsRecords   RequestAction = "infoDnsRecords"
	actionUpdateDnsZone    RequestAction = "updateDnsZone"
	actionUpdateDnsRecords RequestAction = "updateDnsRecords"
)

// Holder for Netcup DNS client context.
type NetcupDnsClient struct {
	customerNumber  int
	apiKey          string
	apiPassword     string
	clientRequestId string
	apiEndpoint     string
	retryConfig     *RetryConfig
	circuitBreaker  *CircuitBreaker
	httpClient      *http.Client
}

// RetryConfig holds retry and backoff configuration
type RetryConfig struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	mu              sync.RWMutex
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	threshold       int           // consecutive failures to open circuit
	timeout         time.Duration // how long to wait before half-open
	halfOpenMaxReqs int           // max requests to allow in half-open state
}

// ErrCircuitOpen is returned when circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ErrRateLimitExceeded is returned when rate limit is hit
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// Additional optional flags for client creation
type NetcupDnsClientOptions struct {
	ClientRequestId string
	ApiEndpoint     string // useful for testing
	RetryConfig     *RetryConfig
	CircuitBreaker  *CircuitBreaker
	HTTPClient      *http.Client
}

// Netcup session context object to hold session information, like apiSessionId or last response.
type NetcupSession struct {
	apiSessionId   string
	apiKey         string
	customerNumber int
	endpoint       string
	LastResponse   *NetcupBaseResponseMessage
	client         *NetcupDnsClient
}

// DnsZoneData holds information about a DNS zone of a domain.
type DnsZoneData struct {
	DomainName   string `json:"name"`
	Ttl          string `json:"ttl"`
	Serial       string `json:"serial"`
	Refresh      string `json:"refresh"`
	Retry        string `json:"retry"`
	Expire       string `json:"expire"`
	DnsSecStatus bool   `json:"dnssecstatus"`
}

// DnsRecord holds information about a single DNS record entry.
type DnsRecord struct {
	Id           string `json:"id"`
	Hostname     string `json:"hostname"`
	Type         string `json:"type"`
	Priority     string `json:"priority"`
	Destination  string `json:"destination"`
	DeleteRecord bool   `json:"deleterecord"`
	State        string `json:"state"`
}

// Response message, as defined by the Netcup API. This is intentionally not complete,
// because the responseData can vary by any sub type of message.
type NetcupBaseResponseMessage struct {
	ServerRequestId string `json:"serverrequestid"`
	ClientRequestId string `json:"clientrequestid"`
	Action          string `json:"action"`
	Status          string `json:"status"`
	StatusCode      int    `json:"statuscode"`
	ShortMessage    string `json:"shortmessage"`
	LongMessage     string `json:"longmessage"`
}

// Parameters used for login() request. These are special in the way they don't
// contain apisessionid field and contain apipassword initially.
type LoginParams struct {
	CustomerNumber  int    `json:"customernumber"`
	ApiKey          string `json:"apikey"`
	ApiPassword     string `json:"apipassword"`
	ClientRequestId string `json:"clientrequestid"`
}

// Payload used for login request
type LoginPayload struct {
	Action RequestAction `json:"action"`
	Params *LoginParams  `json:"param"`
}

// Base payload for all API requests, except for login().
type BasePayload struct {
	Action RequestAction     `json:"action"`
	Params *NetcupBaseParams `json:"param"`
}

// This is what Netcup expects to be in "params" (except for login() request, which doesn't have ApiSessionId)
type NetcupBaseParams struct {
	CustomerNumber  int    `json:"customernumber"`
	ApiSessionId    string `json:"apisessionid"`
	ApiKey          string `json:"apikey"`
	ClientRequestId string `json:"clientrequestid"`
}

// Inner resonse data of a login response.
type LoginResponseData struct {
	ApiSessionId string `json:"apisessionid"`
}

// Response payload of a login response.
type LoginResponsePayload struct {
	NetcupBaseResponseMessage
	ResponseData *LoginResponseData `json:"responsedata"`
}

// Inner response data of InfoDnsZone response.
type InfoDnsZoneResponsePayload struct {
	NetcupBaseResponseMessage
	ResponseData *DnsZoneData `json:"responsedata,omitempty"`
}

// Parameters for InfoDnsZone request
type InfoDnsZoneParams struct {
	NetcupBaseParams
	DomainName string `json:"domainname"`
}

// Payload for InfoDnsZone request
type InfoDnsZonePayload struct {
	Action RequestAction      `json:"action"`
	Params *InfoDnsZoneParams `json:"param"`
}

// Parameters for InfoDnsRecords
type InfoDnsRecordsParams InfoDnsZoneParams

// Payload for InfoDnsRecords request
type InfoDnsRecordsPayload struct {
	Action RequestAction         `json:"action"`
	Params *InfoDnsRecordsParams `json:"param"`
}

type InfoDnsRecordsResponseData struct {
	DnsRecords []DnsRecord `json:"dnsrecords"`
}
type InfoDnsRecordsResponsePayload struct {
	NetcupBaseResponseMessage
	ResponseData *InfoDnsRecordsResponseData `json:"responsedata"`
}

type UpdateDnsZoneParams struct {
	NetcupBaseParams
	DomainName string       `json:"domainname"`
	DnsZone    *DnsZoneData `json:"dnszone"`
}

// Payload for UpdateDnsZone request
type UpdateDnsZonePayload struct {
	Action RequestAction        `json:"action"`
	Params *UpdateDnsZoneParams `json:"param"`
}

type UpdateDnsZoneResponsePayload InfoDnsZoneResponsePayload

type DnsRecordSet struct {
	Content []DnsRecord `json:"dnsrecords"`
}

type UpdateDnsRecordsParams struct {
	NetcupBaseParams
	DomainName string        `json:"domainname"`
	DnsRecords *DnsRecordSet `json:"dnsrecordset"`
}

type UpdateDnsRecordsPayload struct {
	Action RequestAction           `json:"action"`
	Params *UpdateDnsRecordsParams `json:"param"`
}

type UpdateDnsRecordsResponseData struct {
	DnsRecords []DnsRecord `json:"dnsrecords"`
}

// Response payload sent by Netcup upon updateDnsRecords() request
type UpdateDnsRecordsResponsePayload struct {
	NetcupBaseResponseMessage
	ResponseData *UpdateDnsRecordsResponseData `json:"responsedata"`
}

// Creates a new client to interact with Netcup DNS API.
func NewNetcupDnsClient(customerNumber int, apiKey string, apiPassword string) *NetcupDnsClient {
	return NewNetcupDnsClientWithOptions(customerNumber, apiKey, apiPassword, &NetcupDnsClientOptions{})
}

// Create a new client to interact with Netcup DNS API, using own given clientRequestId.
func NewNetcupDnsClientWithOptions(customerNumber int, apiKey string, apiPassword string, opts *NetcupDnsClientOptions) *NetcupDnsClient {
	// Set defaults
	retryConfig := &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    1000 * time.Millisecond,
		MaxBackoff:        30000 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
	if opts.RetryConfig != nil {
		retryConfig = opts.RetryConfig
	}

	circuitBreaker := &CircuitBreaker{
		state:           StateClosed,
		threshold:       5,
		timeout:         60 * time.Second,
		halfOpenMaxReqs: 3,
	}
	if opts.CircuitBreaker != nil {
		circuitBreaker = opts.CircuitBreaker
	}

	httpClient := &http.Client{
		Timeout: defaultRequestTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	if opts.HTTPClient != nil {
		httpClient = opts.HTTPClient
	}

	client := &NetcupDnsClient{
		customerNumber: customerNumber,
		apiKey:         apiKey,
		apiPassword:    apiPassword,
		apiEndpoint:    netcupApiEndpointJSON,
		retryConfig:    retryConfig,
		circuitBreaker: circuitBreaker,
		httpClient:     httpClient,
	}

	if opts.ApiEndpoint != "" {
		client.apiEndpoint = opts.ApiEndpoint
	}
	if opts.ClientRequestId != "" {
		client.clientRequestId = opts.ClientRequestId
	}

	return client
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   API Implementation
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Login to Netcup API. Returns a valid NetcupSession or error.
func (c *NetcupDnsClient) Login() (*NetcupSession, error) {
	if buf, err := c.doPostWithRetry(c.apiEndpoint, &LoginPayload{
		Action: actionLogin,
		Params: &LoginParams{
			CustomerNumber:  c.customerNumber,
			ApiKey:          c.apiKey,
			ApiPassword:     c.apiPassword,
			ClientRequestId: c.clientRequestId,
		},
	}); err != nil {
		return nil, err
	} else {
		lr := &LoginResponseData{}
		if br, err := handleResponse("Login", buf, lr); err != nil {
			return nil, err
		} else {
			return &NetcupSession{
				apiSessionId:   lr.ApiSessionId,
				apiKey:         c.apiKey,
				customerNumber: c.customerNumber,
				endpoint:       c.apiEndpoint,
				LastResponse:   br,
				client:         c,
			}, nil
		}
	}
}

// Query information about DNS zone.
func (s *NetcupSession) InfoDnsZone(domainName string) (*DnsZoneData, error) {
	if buf, err := s.client.doPostWithRetry(s.endpoint, &InfoDnsZonePayload{
		Action: actionInfoDnsZone,
		Params: &InfoDnsZoneParams{
			NetcupBaseParams: NetcupBaseParams{
				CustomerNumber:  s.customerNumber,
				ApiKey:          s.apiKey,
				ApiSessionId:    s.apiSessionId,
				ClientRequestId: s.LastResponse.ClientRequestId,
			},
			DomainName: domainName,
		},
	}); err != nil {
		return nil, err
	} else {
		respData := &DnsZoneData{}
		if br, err := handleResponse("InfoDnsZone", buf, respData); err != nil {
			if br != nil {
				s.LastResponse = br
			}
			return nil, err
		} else {
			s.LastResponse = br
			return respData, nil
		}
	}
}

// Query information about all DNS records.
func (s *NetcupSession) InfoDnsRecords(domainName string) (*[]DnsRecord, error) {
	emptyRecs := make([]DnsRecord, 0)
	if buf, err := s.client.doPostWithRetry(s.endpoint, &InfoDnsRecordsPayload{
		Action: actionInfoDnsRecords,
		Params: &InfoDnsRecordsParams{
			NetcupBaseParams: NetcupBaseParams{
				CustomerNumber:  s.customerNumber,
				ApiKey:          s.apiKey,
				ApiSessionId:    s.apiSessionId,
				ClientRequestId: s.LastResponse.ClientRequestId,
			},
			DomainName: domainName,
		},
	}); err != nil {
		return &emptyRecs, err
	} else {
		respData := &InfoDnsRecordsResponseData{
			DnsRecords: emptyRecs,
		}
		if br, err := handleResponse("InfoDnsRecords", buf, respData); err != nil {
			if br != nil {
				s.LastResponse = br
			}
			return &emptyRecs, err
		} else {
			s.LastResponse = br
			return &respData.DnsRecords, nil
		}
	}
}

// Update data of a DNS zone, returning an updated DnsZoneData.
func (s *NetcupSession) UpdateDnsZone(domainName string, dnsZone *DnsZoneData) (*DnsZoneData, error) {
	if buf, err := s.client.doPostWithRetry(s.endpoint, &UpdateDnsZonePayload{
		Action: actionUpdateDnsZone,
		Params: &UpdateDnsZoneParams{
			NetcupBaseParams: NetcupBaseParams{
				CustomerNumber:  s.customerNumber,
				ApiKey:          s.apiKey,
				ApiSessionId:    s.apiSessionId,
				ClientRequestId: s.LastResponse.ClientRequestId,
			},
			DomainName: domainName,
			DnsZone:    dnsZone,
		},
	}); err != nil {
		return nil, err
	} else {
		respData := &DnsZoneData{}
		if br, err := handleResponse("UpdateDnsZone", buf, respData); err != nil {
			if br != nil {
				s.LastResponse = br
			}
			return nil, err
		} else {
			s.LastResponse = br
			return respData, nil
		}
	}
}

// Update set of DNS records for a given domain name, returning updated DNS records.
func (s *NetcupSession) UpdateDnsRecords(domainName string, dnsRecordSet *[]DnsRecord) (*[]DnsRecord, error) {
	emptyRecs := make([]DnsRecord, 0)
	if buf, err := s.client.doPostWithRetry(s.endpoint, &UpdateDnsRecordsPayload{
		Action: actionUpdateDnsRecords,
		Params: &UpdateDnsRecordsParams{
			NetcupBaseParams: NetcupBaseParams{
				CustomerNumber:  s.customerNumber,
				ApiKey:          s.apiKey,
				ApiSessionId:    s.apiSessionId,
				ClientRequestId: s.LastResponse.ClientRequestId,
			},
			DomainName: domainName,
			DnsRecords: &DnsRecordSet{
				Content: *dnsRecordSet,
			},
		},
	}); err != nil {
		return &emptyRecs, err
	} else {
		respData := &UpdateDnsRecordsResponseData{
			DnsRecords: emptyRecs,
		}
		if br, err := handleResponse("UpdateDnsRecords", buf, respData); err != nil {
			if br != nil {
				s.LastResponse = br
			}
			return &emptyRecs, err
		} else {
			s.LastResponse = br
			return &respData.DnsRecords, nil
		}
	}
}

// Logout from active Netcup session. This may return an error (which can be ignored).
func (s *NetcupSession) Logout() error {
	req := &BasePayload{
		Action: actionLogout,
		Params: &NetcupBaseParams{
			CustomerNumber:  s.customerNumber,
			ApiSessionId:    s.apiSessionId,
			ApiKey:          s.apiKey,
			ClientRequestId: s.LastResponse.ClientRequestId,
		},
	}
	// logout is always assumed successful response, but we need to check for technical errors here.
	if _, err := s.client.doPostWithRetry(s.endpoint, req); err != nil {
		return err
	}
	return nil
}

// Stringer implementation for NetcupSession.
func (s *NetcupSession) String() string {
	return fmt.Sprintf(
		"{ "+
			"\"apiSessionId\": \"%s\", "+
			"\"LastResponse\": %v "+
			"}",
		s.apiSessionId,
		s.LastResponse,
	)
}

// Stringer implementation for NetcupBaseResponseMessage.
func (r *NetcupBaseResponseMessage) String() string {
	return fmt.Sprintf(
		"{ "+
			"\"ServerRequestId\": \"%s\", "+
			"\"ClientRequestId\": \"%s\", "+
			"\"Action\": \"%s\", "+
			"\"Status\": \"%s\", "+
			"\"StatusCode\": \"%d\", "+
			"\"ShortMessage\": \"%s\", "+
			"\"LongMessage\": \"%s\" "+
			"}",
		r.ServerRequestId,
		r.ClientRequestId,
		r.Action,
		r.Status,
		r.StatusCode,
		r.ShortMessage,
		r.LongMessage,
	)
}

// Stringer implementation for DnsZoneData.
func (d *DnsZoneData) String() string {
	return fmt.Sprintf(
		"{ "+
			"\"DomainName\": \"%s\", "+
			"\"Ttl\": \"%s\", "+
			"\"Serial\": \"%s\", "+
			"\"Refresh\": \"%s\", "+
			"\"Retry\": \"%s\", "+
			"\"Expire\": \"%s\", "+
			"\"DnsSecStatus\": %v "+
			"}",
		d.DomainName,
		d.Ttl,
		d.Serial,
		d.Refresh,
		d.Retry,
		d.Expire,
		d.DnsSecStatus,
	)
}

// Stringer implementation for DnsRecord
func (d *DnsRecord) String() string {
	return fmt.Sprintf(
		"{ "+
			"\"Id\": \"%s\", "+
			"\"Hostname\": \"%s\", "+
			"\"Type\": \"%s\", "+
			"\"Priority\": \"%s\", "+
			"\"Destination\": \"%s\", "+
			"\"DeleteRecord\": %v, "+
			"\"State\": \"%s\" "+
			"}",
		d.Id,
		d.Hostname,
		d.Type,
		d.Priority,
		d.Destination,
		d.DeleteRecord,
		d.State,
	)
}

func handleResponse(reqType string, buf *bytes.Buffer, respData interface{}) (*NetcupBaseResponseMessage, error) {
	type ReadResponse struct {
		NetcupBaseResponseMessage
		// response data may be empty string, or of any type so we need to be careful here
		ResponseData interface{} `json:"responsedata"`
	}
	resp := &ReadResponse{}
	dec := json.NewDecoder(buf)
	if err := dec.Decode(resp); err != nil {
		return nil, err
	}
	if resp.Status == string(StatusError) {
		return &resp.NetcupBaseResponseMessage, fmt.Errorf("%s failed: (%d) '%s' '%s' '%s'",
			reqType, resp.StatusCode, resp.Status, resp.ShortMessage, resp.LongMessage)
	}
	// try to convert the responseData to the target type
	b, err := json.Marshal(resp.ResponseData)
	if err != nil {
		return &resp.NetcupBaseResponseMessage, err
	}
	err = json.Unmarshal(b, respData)
	return &resp.NetcupBaseResponseMessage, err
}

// Circuit breaker methods

// NewCircuitBreaker creates a new circuit breaker with given parameters
func NewCircuitBreaker(threshold int, timeout time.Duration, halfOpenMaxReqs int) *CircuitBreaker {
	return &CircuitBreaker{
		state:           StateClosed,
		threshold:       threshold,
		timeout:         timeout,
		halfOpenMaxReqs: halfOpenMaxReqs,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition from open to half-open
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.timeout {
		cb.state = StateHalfOpen
		cb.successCount = 0
		cb.failureCount = 0
	}

	// If circuit is open, fail fast
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}

	// If half-open, check if we've exceeded the request limit
	if cb.state == StateHalfOpen && cb.successCount+cb.failureCount >= cb.halfOpenMaxReqs {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

func (cb *CircuitBreaker) onSuccess() {
	if cb.state == StateHalfOpen {
		cb.successCount++
		// If we've had enough successes in half-open, close the circuit
		if cb.successCount >= cb.halfOpenMaxReqs {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	} else if cb.state == StateClosed {
		// Reset failure count on success
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen {
		// Any failure in half-open state reopens the circuit
		cb.state = StateOpen
		cb.successCount = 0
	} else if cb.state == StateClosed && cb.failureCount >= cb.threshold {
		// Too many failures, open the circuit
		cb.state = StateOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry circuit breaker open errors
	if errors.Is(err, ErrCircuitOpen) {
		return false
	}

	// Check for network errors (timeout, connection refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check error message for rate limiting or temporary issues
	errMsg := err.Error()
	if containsAny(errMsg, []string{"rate limit", "too many requests", "429", "503", "timeout"}) {
		return true
	}

	return false
}

// isRateLimitError checks if an error is due to rate limiting
func isRateLimitError(err error, statusCode int) bool {
	if statusCode == 429 {
		return true
	}
	if err != nil {
		errMsg := err.Error()
		return containsAny(errMsg, []string{"rate limit", "too many requests", "429"})
	}
	return false
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// calculateBackoff calculates the backoff duration for a given retry attempt
func (rc *RetryConfig) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.InitialBackoff
	}

	backoff := float64(rc.InitialBackoff) * math.Pow(rc.BackoffMultiplier, float64(attempt))
	if backoff > float64(rc.MaxBackoff) {
		backoff = float64(rc.MaxBackoff)
	}

	return time.Duration(backoff)
}

// internal helper for doing HTTP post with given payload, retry logic, and circuit breaker.
func (c *NetcupDnsClient) doPostWithRetry(endpoint string, payload interface{}) (*bytes.Buffer, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		// Use circuit breaker to protect the call
		err := c.circuitBreaker.Call(func() error {
			buf, err := c.doPost(endpoint, payload)
			if err != nil {
				lastErr = err
				return err
			}
			// Success - store the buffer in lastErr as a special marker
			lastErr = &successMarker{buf: buf}
			return nil
		})

		// Check for successful result
		if marker, ok := lastErr.(*successMarker); ok {
			return marker.buf, nil
		}

		// Circuit breaker is open - fail fast without retry
		if errors.Is(err, ErrCircuitOpen) {
			return nil, fmt.Errorf("circuit breaker open after %d attempts: %w", attempt, lastErr)
		}

		// If we're out of retries, return the error
		if attempt >= c.retryConfig.MaxRetries {
			break
		}

		// Check if error is retryable
		if !isRetryableError(lastErr) {
			return nil, lastErr
		}

		// Calculate backoff with jitter for rate limiting
		backoff := c.retryConfig.calculateBackoff(attempt)

		// Add extra delay for rate limit errors
		if containsAny(lastErr.Error(), []string{"rate limit", "429"}) {
			backoff = backoff * 2 // Double the backoff for rate limits
		}

		// Sleep before retry
		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.retryConfig.MaxRetries, lastErr)
}

// successMarker is used to pass successful buffer result through circuit breaker
type successMarker struct {
	buf *bytes.Buffer
}

func (s *successMarker) Error() string {
	return "success"
}

// doPost performs the actual HTTP POST request
func (c *NetcupDnsClient) doPost(endpoint string, payload interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", netcupApiContentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var b bytes.Buffer
		if n, err := b.ReadFrom(resp.Body); err == nil && n > 0 {
			respErr := fmt.Errorf("unexpected error code: %d, response: %s", resp.StatusCode, b.String())

			// Check for rate limiting
			if isRateLimitError(respErr, resp.StatusCode) {
				return nil, fmt.Errorf("%w: %v", ErrRateLimitExceeded, respErr)
			}

			return nil, respErr
		}
		return nil, fmt.Errorf("unexpected error code: %d", resp.StatusCode)
	}

	buf.Reset()
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	return &buf, nil
}
