package cho

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Exchange struct {
	*http.Request
	Response http.ResponseWriter
}

func (c *Exchange) MustQueryInt(key string) int {
	val, err := strconv.Atoi(c.Request.URL.Query().Get(key))
	if err != nil {
		panic(fmt.Sprintf(`"%s" must be int`, key))
	}
	return val
}

func (c *Exchange) MustQueryInt64(key string) int64 {
	val, err := strconv.ParseInt(c.Request.URL.Query().Get(key), 10, 64)
	if err != nil {
		panic(fmt.Sprintf(`"%s" must be int64`, key))
	}
	return val
}

func (c *Exchange) Fail(code int, msg string, data any) {
	_ = c.Json(http.StatusBadRequest, map[string]any{
		"code":    code,
		"message": msg,
		"data":    data,
		"ts":      time.Now().Unix(),
	})
}

func (c *Exchange) Success(data any) {
	_ = c.Json(http.StatusOK, map[string]any{
		"code":    0,
		"message": "success",
		"data":    data,
		"ts":      time.Now().Unix(),
	})
}

func (c *Exchange) Json(code int, data any) error {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(code)

	encoder := json.NewEncoder(c.Response)
	return encoder.Encode(data)
}

func (e *Exchange) CheckedBody(dest any) error {
	decoder := json.NewDecoder(e.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return err
	}

	if err := validate.Struct(dest); err != nil {
		return err
	}

	return nil
}
