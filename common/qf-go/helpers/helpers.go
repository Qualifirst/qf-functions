package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func GraphQLQuery(ctx context.Context, url string, authHeader string, authToken string, query string, variables map[string]any) (any, error) {
	requestBody, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return nil, fmt.Errorf("error marshalling GraphQL request:\n>>> %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating GraphQL query request:\n>>> %w", err)
	}
	request.Header.Add("Content-Type", "application/json")
	if authHeader != "" && authToken != "" {
		request.Header.Add(authHeader, authToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error requesting GraphQL query:\n>>> %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading GraphQL query response:\n>>> %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from GraphQL query: [%s] %s", response.Status, responseBody)
	}

	var responseJsonAny any
	err = json.Unmarshal(responseBody, &responseJsonAny)
	if err != nil {
		return nil, fmt.Errorf("invalid response format from GraphQL query: [%s] %s", response.Status, responseBody)
	}

	return responseJsonAny, nil
}

func TempEnvVars(vars map[string]string) (reset func()) {
	current := map[string]string{}
	for key, val := range vars {
		current[key] = os.Getenv(key)
		os.Setenv(key, val)
	}
	return func() {
		for key, val := range current {
			os.Setenv(key, val)
		}
	}
}

func StringPtr(s string) *string {
	return &s
}

func traverse[T any](obj any, key any, keys []any, fallback T) (T, error) {
	reflectedObj := reflect.ValueOf(obj)
	switch reflectedObj.Kind() {
	case reflect.Slice, reflect.Array:
		typedKey, ok := key.(int)
		if !ok {
			return fallback, fmt.Errorf("expected int key for %T, got %T", obj, key)
		}
		if typedKey >= reflectedObj.Len() {
			return fallback, fmt.Errorf("index %v out of range %v", typedKey, reflectedObj.Len()-1)
		}
		val := reflectedObj.Index(typedKey).Interface()
		if len(keys) > 0 {
			return traverse(val, keys[0], keys[1:], fallback)
		}
		typedVal, ok := val.(T)
		if !ok {
			var empty T
			return fallback, fmt.Errorf("could not type assert final value %T into %T", val, empty)
		}
		return typedVal, nil
	case reflect.Map:
		typedKey, ok := key.(string)
		if !ok {
			return fallback, fmt.Errorf("expected string key for %T, got %T (%v)", obj, key, key)
		}
		res := reflectedObj.MapIndex(reflect.ValueOf(typedKey))
		if !res.IsValid() {
			return fallback, fmt.Errorf("key %s not found", typedKey)
		}
		val := res.Interface()
		if len(keys) > 0 {
			return traverse(val, keys[0], keys[1:], fallback)
		}
		typedVal, ok := val.(T)
		if !ok {
			var empty T
			return fallback, fmt.Errorf("could not type assert final value %T into %T", val, empty)
		}
		return typedVal, nil
	}
	return fallback, fmt.Errorf("cannot traverse object of type %T", obj)
}

func TraverseWithError[T any](obj any, keys []any, fallback T) (T, error) {
	return traverse(obj, keys[0], keys[1:], fallback)
}

func Traverse[T any](obj any, keys []any, fallback T) T {
	res, _ := TraverseWithError(obj, keys, fallback)
	return res
}

// normalizeString removes diacritics/accents and converts the string to lowercase.
func normalizeString(s string) (string, error) {
	// The transform.Chain combines several transformers.
	// norm.NFD: Decomposes characters (e.g., 'é' becomes 'e' + '´').
	// runes.Remove(runes.In(unicode.Mn)): Removes non-spacing marks (the accents).
	// norm.NFC: Recomposes characters back to their pre-composed form.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	// transform.String applies the transformer t to the input string s.
	result, _, err := transform.String(t, s)
	if err != nil {
		return "", fmt.Errorf("error normalizing string\nERROR=%w", err)
	}
	return result, nil
}

// CompareStrings checks if two strings are equal, ignoring case and accents.
func CompareStrings(s1, s2 string) (bool, error) {
	n1, err := normalizeString(s1)
	if err != nil {
		return false, fmt.Errorf("could not normalize s1\nERROR=%w", err)
	}
	n2, err := normalizeString(s2)
	if err != nil {
		return false, fmt.Errorf("could not normalize s2\nERROR=%w", err)
	}
	// strings.EqualFold performs a case-insensitive comparison.
	return strings.EqualFold(n1, n2), nil
}

// StringInSlice checks if s is in l, ignoring case and accents.
func StringInSlice(s string, l []string) (bool, error) {
	n, err := normalizeString(s)
	if err != nil {
		return false, fmt.Errorf("error normalizing string\nERROR=%w", err)
	}
	for _, sl := range l {
		nl, err := normalizeString(sl)
		if err != nil {
			return false, fmt.Errorf("error normalizing string from slice\nERROR=%w", err)
		}
		if strings.EqualFold(n, nl) {
			return true, nil
		}
	}
	return false, nil
}

func PrintlnJson(args ...any) {
	argsJson := make([]any, len(args))
	for i, arg := range args {
		argJson, err := json.MarshalIndent(arg, "", "  ")
		if err != nil {
			argsJson[i] = arg
		} else {
			argsJson[i] = string(argJson)
		}
	}
	fmt.Println(argsJson...)
}
