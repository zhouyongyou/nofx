package logger

import (
	"fmt"
	"time"
)

// safeFloat64 安全地從 map 中提取 float64 值
func safeFloat64(m map[string]interface{}, key string) (float64, error) {
	v, exists := m[key]
	if !exists {
		return 0, fmt.Errorf("key '%s' not found", key)
	}

	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("key '%s' has unexpected type %T", key, v)
	}
}

// safeString 安全地從 map 中提取 string 值
func safeString(m map[string]interface{}, key string) (string, error) {
	v, exists := m[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found", key)
	}

	str, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("key '%s' is not a string (type: %T)", key, v)
	}
	return str, nil
}

// safeInt 安全地從 map 中提取 int 值
func safeInt(m map[string]interface{}, key string) (int, error) {
	v, exists := m[key]
	if !exists {
		return 0, fmt.Errorf("key '%s' not found", key)
	}

	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	default:
		return 0, fmt.Errorf("key '%s' has unexpected type %T", key, v)
	}
}

// safeTime 安全地從 map 中提取 time.Time 值
func safeTime(m map[string]interface{}, key string) (time.Time, error) {
	v, exists := m[key]
	if !exists {
		return time.Time{}, fmt.Errorf("key '%s' not found", key)
	}

	t, ok := v.(time.Time)
	if !ok {
		return time.Time{}, fmt.Errorf("key '%s' is not a time.Time (type: %T)", key, v)
	}
	return t, nil
}

// safeFloat64OrDefault 安全地提取 float64，失敗時返回默認值
func safeFloat64OrDefault(m map[string]interface{}, key string, defaultValue float64) float64 {
	val, err := safeFloat64(m, key)
	if err != nil {
		return defaultValue
	}
	return val
}

// safeIntOrDefault 安全地提取 int，失敗時返回默認值
func safeIntOrDefault(m map[string]interface{}, key string, defaultValue int) int {
	val, err := safeInt(m, key)
	if err != nil {
		return defaultValue
	}
	return val
}
