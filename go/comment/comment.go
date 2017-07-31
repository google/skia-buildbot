package comment

/*
	Common implementation of a comment.
*/

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"time"
)

// comment makes up the raw contents of a Comment.
type comment struct {
	Id        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
}

// Comment is a struct containing a comment by a user on a given entity.
type Comment struct {
	comment
}

// Return a Comment instance.
func New(id, msg, user string) *Comment {
	return &Comment{
		comment{
			Id:        id,
			Message:   msg,
			Timestamp: time.Now(),
			User:      user,
		},
	}
}

// See docs for json.Marshaler.
func (c *Comment) MarshalJSON() ([]byte, error) {
	c.Timestamp = c.Timestamp.UTC()
	return json.Marshal(c.comment)
}

// See docs for json.Unmarshaler.
func (c *Comment) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &c.comment); err != nil {
		return err
	}
	c.Timestamp = c.Timestamp.UTC()
	return nil
}

// See docs for gob.GobEncoder.
func (c *Comment) GobEncode() ([]byte, error) {
	c.Timestamp = c.Timestamp.UTC()
	buf := bytes.NewBuffer([]byte{})
	if err := gob.NewEncoder(buf).Encode(c.comment); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// See docs for gob.GobDecoder.
func (c *Comment) GobDecode(b []byte) error {
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c.comment); err != nil {
		return err
	}
	c.Timestamp = c.Timestamp.UTC()
	return nil
}
