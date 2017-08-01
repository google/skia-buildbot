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

// Comment is a struct containing a comment by a user on a given entity.
type Comment struct {
	Id        string
	Message   string
	Timestamp time.Time
	User      string
}

// Return a Comment instance.
func New(id, msg, user string) *Comment {
	return &Comment{
		Id:        id,
		Message:   msg,
		Timestamp: time.Now().UTC(),
		User:      user,
	}
}

// Return a copy of the Comment.
func (c *Comment) Copy() *Comment {
	return &Comment{
		Id:        c.Id,
		Message:   c.Message,
		Timestamp: c.Timestamp,
		User:      c.User,
	}
}

// comment makes up the raw contents of a Comment.
type comment struct {
	Id        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
}

// See docs for json.Marshaler.
func (c *Comment) MarshalJSON() ([]byte, error) {
	c.Timestamp = c.Timestamp.UTC()
	return json.Marshal(comment{
		Id:        c.Id,
		Message:   c.Message,
		Timestamp: c.Timestamp,
		User:      c.User,
	})
}

// See docs for json.Unmarshaler.
func (c *Comment) UnmarshalJSON(b []byte) error {
	var body comment
	if err := json.Unmarshal(b, &body); err != nil {
		return err
	}
	c.Id = body.Id
	c.Message = body.Message
	c.Timestamp = body.Timestamp.UTC()
	c.User = body.User
	return nil
}

// See docs for gob.GobEncoder.
func (c *Comment) GobEncode() ([]byte, error) {
	c.Timestamp = c.Timestamp.UTC()
	buf := bytes.NewBuffer([]byte{})
	if err := gob.NewEncoder(buf).Encode(comment{
		Id:        c.Id,
		Message:   c.Message,
		Timestamp: c.Timestamp,
		User:      c.User,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// See docs for gob.GobDecoder.
func (c *Comment) GobDecode(b []byte) error {
	var body comment
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&body); err != nil {
		return err
	}
	c.Id = body.Id
	c.Message = body.Message
	c.Timestamp = body.Timestamp.UTC()
	c.User = body.User
	return nil
}
