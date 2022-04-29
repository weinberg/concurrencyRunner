package config

import "encoding/json"

/**************************************
 * ActionTypeEnum Enum
 **************************************/

type ActionTypeEnum int

const (
	ActionTypeUnknown ActionTypeEnum = iota
	ActionTypeRun
	ActionTypePause
	ActionTypeContinue
)

func (t ActionTypeEnum) String() string {
	return [...]string{"unknown", "run", "pause", "continue"}[t]
}

func (t *ActionTypeEnum) FromString(Action string) ActionTypeEnum {
	return map[string]ActionTypeEnum{
		"unknown":  ActionTypeUnknown,
		"run":      ActionTypeRun,
		"pause":    ActionTypePause,
		"continue": ActionTypeContinue,
	}[Action]
}

func (t ActionTypeEnum) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *ActionTypeEnum) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	*t = t.FromString(s)
	return nil
}

/**************************************
 * AdapterEnum
 **************************************/

type AdapterEnum int

const (
	AdapterUnknown AdapterEnum = iota
	AdapterDelve
)

func (t AdapterEnum) String() string {
	return [...]string{"Unknown", "delve"}[t]
}

func (t *AdapterEnum) FromString(Adapter string) AdapterEnum {
	return map[string]AdapterEnum{
		"Unknown": AdapterUnknown,
		"delve":   AdapterDelve,
	}[Adapter]
}

func (t AdapterEnum) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *AdapterEnum) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	*t = t.FromString(s)
	return nil
}
