package api

import (
	"encoding/json"
	"log"
	"net/http"
)

type apiResponse struct {
	Code    int         `json:"code"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type apiError struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Response writes a JSON response to the given response writer.
func Response(w http.ResponseWriter, data interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	encoder := json.NewEncoder(w)
	err := encoder.Encode(&apiResponse{
		Code:    code,
		Success: true,
		Data:    data,
	})

	if err != nil {
		log.Println("error occurred encoding JSON Response: ", err)
	}
}

// Error writes an message (as JSON) to the given http writer and sends the given response code to the client.
func Error(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	encoder := json.NewEncoder(w)
	err := encoder.Encode(&apiError{
		Code:    code,
		Success: false,
		Message: message,
	})

	if err != nil {
		log.Println("error occurred encoding JSON Response (err): ", err)
	}
}

// RequireJSONBody is a helper function for unmarshalling a JSON body if it is valid
// or returning the right errors to the client if it is not valid.
func RequireJSONBody(w http.ResponseWriter, r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(target)
	if err != nil {
		Error(w, "Body was not valid JSON or field types are not correct.", http.StatusBadRequest)
		return err
	}
	return nil
}
