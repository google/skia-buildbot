// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"errors"
	"fmt"
	"html/template"
	"sync"

	"golang.org/x/net/context"
)

// UIPage controls how some settings section (usually corresponding to a key in
// global settings JSON blob) is displayed and edited in UI.
//
// Packages that wishes to expose UI for managing their settings register a page
// via RegisterUIPage(...) call during init() time.
type UIPage interface {
	// Title is used in UI to name this settings page.
	Title(c context.Context) (string, error)

	// Overview is optional HTML paragraph describing this settings page.
	Overview(c context.Context) (template.HTML, error)

	// Fields describes the schema of this settings page.
	Fields(c context.Context) ([]UIField, error)

	// ReadSettings returns a map "field ID => field value to display".
	//
	// It is called when rendering the settings page.
	ReadSettings(c context.Context) (map[string]string, error)

	// WriteSettings saves settings described as a map "field ID => field value".
	//
	// All values are validated using field validators first.
	WriteSettings(c context.Context, values map[string]string, who, why string) error
}

// UIField is description of a single UI element of the settings page.
//
// Its ID acts as a key in map used by ReadSettings\WriteSettings.
type UIField struct {
	ID             string             // page unique ID
	Title          string             // human friendly name
	Type           UIFieldType        // how the field is displayed and behaves
	Placeholder    string             // optional placeholder value
	Validator      func(string) error // optional value validation
	Help           template.HTML      // optional help text
	ChoiceVariants []string           // valid only for UIFieldChoice
}

// UIFieldType describes look and feel of UI field, see the enum below.
type UIFieldType string

// Note: exact values here are important. They are referenced in the HTML
// template that renders the settings page. See server/settings/admin/*.
const (
	UIFieldText   UIFieldType = "text"   // one line of text, editable
	UIFieldChoice UIFieldType = "choice" // pick one of predefined choices
	UIFieldStatic UIFieldType = "static" // one line of text, read only
)

// IsEditable returns true for fields that can be edited.
func (f UIFieldType) IsEditable() bool {
	return f != UIFieldStatic
}

// BaseUIPage can be embedded into UIPage implementers to provide default
// behavior.
type BaseUIPage struct{}

// Title is used in UI to name this settings block.
func (BaseUIPage) Title(c context.Context) (string, error) {
	return "Untitled settings", nil
}

// Overview is optional HTML paragraph describing this settings block.
func (BaseUIPage) Overview(c context.Context) (template.HTML, error) {
	return "", nil
}

// Fields describes the schema of the config page.
func (BaseUIPage) Fields(c context.Context) ([]UIField, error) {
	return nil, errors.New("not implemented")
}

// ReadSettings returns a map "field ID => field value to display".
func (BaseUIPage) ReadSettings(c context.Context) (map[string]string, error) {
	return nil, errors.New("not implemented")
}

// WriteSettings saves settings described as a map "field ID => field value".
func (BaseUIPage) WriteSettings(c context.Context, values map[string]string, who, why string) error {
	return errors.New("not implemented")
}

// RegisterUIPage makes exposes UI for a block of settings (identified by given
// unique key).
//
// Should be called once when application starts (e.g. from init() of a package
// that defines the settings). Panics if such key is already registered.
func RegisterUIPage(settingsKey string, p UIPage) {
	registry.registerUIPage(settingsKey, p)
}

// GetUIPages returns a map with all registered pages.
func GetUIPages() map[string]UIPage {
	return registry.getUIPages()
}

////////////////////////////////////////////////////////////////////////////////
// Internal stuff.

var registry pageRegistry

type pageRegistry struct {
	lock  sync.RWMutex
	pages map[string]UIPage
}

func (r *pageRegistry) registerUIPage(settingsKey string, p UIPage) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.pages == nil {
		r.pages = make(map[string]UIPage)
	}
	if existing, _ := r.pages[settingsKey]; existing != nil {
		panic(fmt.Errorf("settings UI page for %s is already registered: %T", settingsKey, existing))
	}
	r.pages[settingsKey] = p
}

func (r *pageRegistry) getUIPages() map[string]UIPage {
	r.lock.RLock()
	defer r.lock.RUnlock()
	cpy := make(map[string]UIPage, len(r.pages))
	for k, v := range r.pages {
		cpy[k] = v
	}
	return cpy
}
