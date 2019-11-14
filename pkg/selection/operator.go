package selection

import (
	"bytes"
	"fmt"
	"strings"
)

type Operator string

const (
	DoesNotExist    Operator = "!"
	Equals          Operator = "="
	EqualString     Operator = "'='"
	DoubleEquals    Operator = "=="
	In              Operator = "IN"
	NotEquals       Operator = "!="
	NotEqualsString Operator = "'!='"
	NotIn           Operator = "NOT IN"
	Exists          Operator = "exists"
	GreaterThan     Operator = ">"
	LessThan        Operator = "<"
	Like            Operator = "LIKE"
	FindIn          Operator = "FIND_IN_SET"
)

// Requirement contains values, a key, and an operator that relates the key and values.
type Requirement struct {
	key      string
	operator Operator
	// In huge majority of cases we have at most one value here.
	// It is generally faster to operate on a single-element slice
	// than on a single-element map, so we have a slice here.
	values interface{}
}

func NewRequirement(key string, operator Operator, values interface{}) Requirement {
	return Requirement{key, operator, values}
}

type Requirements []Requirement

func (r Requirements) Append(requirements ...Requirement) Requirements {
	var res []Requirement
	for index := range r {
		res = append(res, r[index])
	}

	for index := range requirements {
		res = append(res, requirements[index])
	}

	return res
}

func (r Requirements) String() string {
	var res []string
	for _, v := range r {
		con, value := v.Patch()
		res = append(res, strings.Replace(con, "?", fmt.Sprintf("%v", value), -1))
	}
	return strings.Join(res, ";")
}

// String returns a human-readable string that represents this
// Requirement. If called on an invalid Requirement, an error is
// returned. See NewRequirement for creating a valid Requirement.
func (r *Requirement) Patch() (string, interface{}) {
	var buffer bytes.Buffer
	if r.operator == DoesNotExist {
		buffer.WriteString("!")
	}

	if r.operator != FindIn {
		buffer.WriteString(r.key)
	}

	switch r.operator {
	case Equals, EqualString:
		buffer.WriteString("=")
	case DoubleEquals:
		buffer.WriteString("==")
	case NotEquals, NotEqualsString:
		buffer.WriteString("!=")
	case In:
		buffer.WriteString(" IN (")
	case NotIn:
		buffer.WriteString(" NOT IN (")
	case GreaterThan:
		buffer.WriteString(">")
	case LessThan:
		buffer.WriteString("<")
	case Like:
		buffer.WriteString(" LIKE ")
	//case Exists, DoesNotExist:
	//	return buffer.String()
	case FindIn:
		buffer.WriteString(" FIND_IN_SET(")
	}

	if r.operator != FindIn {
		buffer.WriteString("?")
	} else {
		buffer.WriteString(r.key + ",?")
	}

	switch r.operator {
	case In, NotIn, FindIn:
		buffer.WriteString(")")
	case Like:
		return buffer.String(), fmt.Sprintf("%%%v%%", r.values)
	}

	return buffer.String(), r.values
}

type Selector struct {
	queryIndex int
	Query      Requirements
	Page       int
	Pagesize   int
	OrderBy    string
	Select     string
}

func NewSelector(r ...Requirement) Selector {
	s := Selector{}
	if len(r) == 0 {
		return s
	}
	s.AddQuery(r...)
	return s
}

func (s *Selector) Len() int {
	return len(s.Query)
}

func (s *Selector) Patch() (string, interface{}) {
	i := s.queryIndex - 1
	return s.Query[i].Patch()
}

func (s *Selector) NextQuery() bool {
CONTINUE:
	s.queryIndex++
	if len(s.Query) < s.queryIndex {
		return false
	}
	// filter out unassigned conditions
	if s.Query[s.queryIndex-1].values == nil {
		goto CONTINUE
	}
	return true
}

func (s *Selector) AddQuery(r ...Requirement) {
	s.Query = append(s.Query, r...)
}

func (s *Selector) AddOrder(str string) {
	if s.OrderBy != "" {
		s.OrderBy += "," + str
	} else {
		s.OrderBy = str
	}
}

func (s Selector) AddSelect(str string) Selector {
	if s.OrderBy != "" {
		s.OrderBy += "," + str
	} else {
		s.OrderBy = str
	}
	return s
}
