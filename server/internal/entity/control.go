package entity

import (
	"errors"
	"regexp"
)

type ControlType string

const (
	ButtonType ControlType = "button"
	TextType   ControlType = "text"
	NumberType ControlType = "number"
)

type Control interface {
	Name() string
	Type() ControlType
	Verify(v any) bool
	Map() map[string]any
}

var ErrInvalidControlType = errors.New("invalid control type")

// TODO: prevent panic when type asserting fail
func NewControl(c map[string]any) (Control, error) {
	switch c["type"] {
	case string(ButtonType):
		return NewButtonControl(c["name"].(string)), nil
	case string(TextType):
		regexStr := c["regex"].(string)
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, err
		}
		return NewTextControl(c["name"].(string), regex), nil
	case string(NumberType):
		intType := c["int"].(bool)
		min := c["min"].(float64)
		max := c["max"].(float64)
		return NewNumberControl(c["name"].(string), intType, min, max), nil
	default:
		return nil, ErrInvalidControlType
	}
}

func NewButtonControl(name string) Control {
	return &buttonControl{name: name}
}

type buttonControl struct {
	name string
}

func (b *buttonControl) Name() string {
	return b.name
}

func (b *buttonControl) Type() ControlType {
	return ButtonType
}

func (b *buttonControl) Verify(v any) bool {
	_, ok := v.(bool)
	return ok
}

func (b *buttonControl) Map() map[string]any {
	return map[string]any{
		"name": b.name,
		"type": string(ButtonType),
	}
}

func NewTextControl(name string, regex *regexp.Regexp) Control {
	return &textControl{name: name, regex: regex}
}

type textControl struct {
	name  string
	regex *regexp.Regexp
}

func (t *textControl) Name() string {
	return t.name
}

func (t *textControl) Type() ControlType {
	return TextType
}

func (t *textControl) Verify(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}

	return t.regex.MatchString(s)
}

func (t *textControl) Map() map[string]any {
	return map[string]any{
		"name":  t.name,
		"type":  string(TextType),
		"regex": t.regex.String(),
	}
}

func NewNumberControl(name string, intType bool, min, max float64) Control {
	if min > max {
		min, max = max, min
	}
	return &numberControl{name: name, int: intType, min: min, max: max}
}

type numberControl struct {
	name string
	int  bool
	min  float64
	max  float64
}

func (n *numberControl) Name() string {
	return n.name
}

func (n *numberControl) Type() ControlType {
	return NumberType
}

func (n *numberControl) verifyInt(v int64) bool {
	if !n.int {
		return false
	}
	return float64(v) >= n.min && float64(v) <= n.max
}

func (n *numberControl) verifyFloat(v float64) bool {
	if n.int {
		return false
	}
	return v >= n.min && v <= n.max
}

func (n *numberControl) Verify(v any) bool {
	switch v := v.(type) {
	case int:
		return n.verifyInt(int64(v))
	case int8:
		return n.verifyInt(int64(v))
	case int16:
		return n.verifyInt(int64(v))
	case int32:
		return n.verifyInt(int64(v))
	case int64:
		return n.verifyInt(v)

	case uint:
		return n.verifyInt(int64(v))
	case uint8:
		return n.verifyInt(int64(v))
	case uint16:
		return n.verifyInt(int64(v))
	case uint32:
		return n.verifyInt(int64(v))
	case uint64:
		return n.verifyInt(int64(v))

	case float32:
		return n.verifyFloat(float64(v))
	case float64:
		return n.verifyFloat(v)

	default:
		return false
	}
}

func (n *numberControl) Map() map[string]any {
	return map[string]any{
		"name": n.name,
		"type": string(NumberType),
		"int":  n.int,
		"min":  n.min,
		"max":  n.max,
	}
}

type selectOption struct {
	label string
	value string
}

type selectControl struct {
	name    string
	options []selectOption
}

func NewSelectControl(name string, options []selectOption) Control {
	return &selectControl{name: name, options: options}
}

func (s *selectControl) Name() string {
	return s.name
}

func (s *selectControl) Type() ControlType {
	return "select"
}

func (s *selectControl) Verify(v any) bool {
	if _, ok := v.(string); !ok {
		return false
	}

	for _, option := range s.options {
		if option.value == v {
			return true
		}
	}
	return false
}

func (s *selectControl) Map() map[string]any {
	options := make([]map[string]string, len(s.options))
	for i, option := range s.options {
		options[i] = map[string]string{
			"label": option.label,
			"value": option.value,
		}
	}

	return map[string]any{
		"name":    s.name,
		"type":    string(s.Type()),
		"options": options,
	}
}
