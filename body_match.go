package checkbodyplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

//SingleBody contains a single body keypair
type SingleBody struct {
	Name      string   `json:"name,omitempty"`
	Values    []string `json:"values,omitempty"`
	MatchType string   `json:"matchtype,omitempty"`
	Required  *bool    `json:"required,omitempty"`
	Contains  *bool    `json:"contains,omitempty"`
	URLDecode *bool    `json:"urldecode,omitempty"`
}

//ResponseError contains a failuer message
type ResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  int    `json:"status,omitempty"`
}

// Config the plugin configuration.
type Config struct {
	Body     []SingleBody
	Response ResponseError
}

// BodyMatch demonstrates a BodyMatch plugin.
type BodyMatch struct {
	name     string
	next     http.Handler
	body     []SingleBody
	response ResponseError
}

// MatchType defines an enum which can be used to specify the match type for the 'contains' config.
type MatchType string

const (
	//MatchAll requires all values to be matched
	MatchAll MatchType = "all"
	//MatchOne requires only one value to be matched
	MatchOne MatchType = "one"
)

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		Body:     []SingleBody{},
		Response: ResponseError{},
	}
}

// New created a new BodyMatch plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.Body) == 0 {
		return nil, fmt.Errorf("configuration incorrect, missing body")
	}

	for _, vBody := range config.Body {
		if strings.TrimSpace(vBody.Name) == "" {
			return nil, fmt.Errorf("configuration incorrect, missing body name")
		}

		if len(vBody.Values) == 0 {
			return nil, fmt.Errorf("configuration incorrect, missing body values")
		} else {
			for _, value := range vBody.Values {
				if strings.TrimSpace(value) == "" {
					return nil, fmt.Errorf("configuration incorrect, empty value found")
				}
			}
		}

		if !vBody.IsContains() && vBody.MatchType == string(MatchAll) {
			return nil, fmt.Errorf("configuration incorrect for body %v %s", vBody.Name, ", matchall can only be used in combination with 'contains'")
		}

		if strings.TrimSpace(vBody.MatchType) == "" {
			return nil, fmt.Errorf("configuration incorrect, missing match type configuration for body %v", vBody.Name)
		}
	}

	if config.Response.Code == "" {
		config.Response.Code = "1034"
	}

	if config.Response.Message == "" {
		config.Response.Message = "Invalid request."
	}

	if config.Response.Status == 0 {
		config.Response.Status = http.StatusBadRequest
	}

	return &BodyMatch{
		name:     name,
		next:     next,
		body:     config.Body,
		response: config.Response,
	}, nil
}

func (a *BodyMatch) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	bodyValid := true

	var reqBody map[string]string
	json.NewDecoder(req.Body).Decode(&reqBody)

	for _, vBody := range a.body {
		reqBodyVal := reqBody[vBody.Name]

		if vBody.IsContains() && reqBodyVal != "" {
			bodyValid = checkContains(&reqBodyVal, &vBody)
		} else {
			bodyValid = checkRequired(&reqBodyVal, &vBody)
		}

		if !bodyValid {
			break
		}
	}

	if bodyValid {
		a.next.ServeHTTP(rw, req)
	} else {
		s := fmt.Sprintf(`{
			"data": null,
			"error": {
				"code": "%s",
				"message": "%s"
			}
		}`, a.response.Code, a.response.Message)

		http.Error(rw, s, a.response.Status)
	}
}

func checkContains(requestValue *string, vBody *SingleBody) bool {
	matchCount := 0
	for _, value := range vBody.Values {
		if strings.Contains(*requestValue, value) {
			matchCount++
		}
	}

	if matchCount == 0 {
		return false
	} else if vBody.MatchType == string(MatchAll) && matchCount != len(vBody.Values) {
		return false
	}

	return true
}

func checkRequired(requestValue *string, vBody *SingleBody) bool {
	matchCount := 0
	for _, value := range vBody.Values {
		if *requestValue == value {
			matchCount++
		}

		if !vBody.IsRequired() && *requestValue == "" {
			matchCount++
		}
	}

	if matchCount == 0 {
		return false
	}

	return true
}

//IsContains checks whether a body value should contain the configured value
func (s *SingleBody) IsContains() bool {
	if s.Contains == nil || *s.Contains == false {
		return false
	}
	return true
}

//IsRequired checks whether a body is mandatory in the request, defaults to 'true'
func (s *SingleBody) IsRequired() bool {
	if s.Required == nil || *s.Required != false {
		return true
	}
	return false
}
