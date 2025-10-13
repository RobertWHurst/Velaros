package json

// M is shorthand for a map[string]any. It is provided as a convenience for
// defining JSON objects in a more concise manner.
type M map[string]any

// Error represents a JSON wrapped error. If you want to return a JSON wrapped
// error, you can use this type. The JSON response will be
// {"error": "your error message"}.
type Error string

// A FieldError can be used as a response body to indicate that a message
// body failed validation. The response will be a JSON object with the field
// name as the key and the error message as the value.
// A slice of FieldErrors can also be used to return multiple validation
// errors. The response will be a JSON object like { "error": "Validation error",
// "fields": [ { "field1": "error message" }, { "field2": "error message" } ] }.
type FieldError struct {
	Field string
	Error string
}
