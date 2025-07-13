package entity

import "regexp"

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
