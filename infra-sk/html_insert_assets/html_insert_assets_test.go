package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleHTML = `
<!DOCTYPE html>
<html>
	<!--
		Commented out head and body tags should be ignored.
		<head>
			<title>I'm a comment</title>
		</head>
		<body>
			<p>I'm a comment</p>
		</body>
	-->

	<head>
		<title>Example</title>
	</head>

	<body>
		<p>
			Example Go template variable: {% .SomeVariable %}
		</p>

		<p {% .SomeVariable %}>
			Example Go template variable inside a tag.
		</p>

		<p class={% .SomeVariable %}>
			Example Go template variable inside an unquoted attribute.
		</p>

		<p class="foo {% .SomeVariable %} bar">
			Example Go template variable inside a quoted attribute.
		</p>

		{% if .SomeCondition %}
			<p>Example Go template conditional.</p>
		{% else %}
			<p>Something else.</p>
		{% end %}

		<p {% if .SomeCondition %} class="foo" {% else %} class="bar" {% end %}>
			Example Go template conditional inside a tag.
		</p>

		<p class={% if .SomeCondition %}foo{% else %}bar{% end %}>
			Example Go template conditional inside an unquoted attribute.
		</p>

		<p class="foo {% if .SomeCondition %}bar{% else %}baz{% end %} qux">
			Example Go template conditional inside a quoted attribute.
		</p>

		{% range .Items %}
			<p>Example Go template range: {% .ItemName %}</p>
		{% else %}
			<p>No items.</p>
		{% end %}
	</body>
</html>
`

const (
	jsPath  = "/dist/index.js"
	cssPath = "/dist/index.css"
	nonce   = "{% .Nonce %}"
)

const expectedHTMLWithNonce = `
<!DOCTYPE html>
<html>
	<!--
		Commented out head and body tags should be ignored.
		<head>
			<title>I'm a comment</title>
		</head>
		<body>
			<p>I'm a comment</p>
		</body>
	-->

	<head>
		<title>Example</title>
	<link rel="stylesheet" href="/dist/index.css" nonce="{% .Nonce %}"></head>

	<body>
		<p>
			Example Go template variable: {% .SomeVariable %}
		</p>

		<p {% .SomeVariable %}>
			Example Go template variable inside a tag.
		</p>

		<p class={% .SomeVariable %}>
			Example Go template variable inside an unquoted attribute.
		</p>

		<p class="foo {% .SomeVariable %} bar">
			Example Go template variable inside a quoted attribute.
		</p>

		{% if .SomeCondition %}
			<p>Example Go template conditional.</p>
		{% else %}
			<p>Something else.</p>
		{% end %}

		<p {% if .SomeCondition %} class="foo" {% else %} class="bar" {% end %}>
			Example Go template conditional inside a tag.
		</p>

		<p class={% if .SomeCondition %}foo{% else %}bar{% end %}>
			Example Go template conditional inside an unquoted attribute.
		</p>

		<p class="foo {% if .SomeCondition %}bar{% else %}baz{% end %} qux">
			Example Go template conditional inside a quoted attribute.
		</p>

		{% range .Items %}
			<p>Example Go template range: {% .ItemName %}</p>
		{% else %}
			<p>No items.</p>
		{% end %}
	<script src="/dist/index.js" nonce="{% .Nonce %}"></script></body>
</html>
`

const expectedHTMLWithoutNonce = `
<!DOCTYPE html>
<html>
	<!--
		Commented out head and body tags should be ignored.
		<head>
			<title>I'm a comment</title>
		</head>
		<body>
			<p>I'm a comment</p>
		</body>
	-->

	<head>
		<title>Example</title>
	<link rel="stylesheet" href="/dist/index.css"></head>

	<body>
		<p>
			Example Go template variable: {% .SomeVariable %}
		</p>

		<p {% .SomeVariable %}>
			Example Go template variable inside a tag.
		</p>

		<p class={% .SomeVariable %}>
			Example Go template variable inside an unquoted attribute.
		</p>

		<p class="foo {% .SomeVariable %} bar">
			Example Go template variable inside a quoted attribute.
		</p>

		{% if .SomeCondition %}
			<p>Example Go template conditional.</p>
		{% else %}
			<p>Something else.</p>
		{% end %}

		<p {% if .SomeCondition %} class="foo" {% else %} class="bar" {% end %}>
			Example Go template conditional inside a tag.
		</p>

		<p class={% if .SomeCondition %}foo{% else %}bar{% end %}>
			Example Go template conditional inside an unquoted attribute.
		</p>

		<p class="foo {% if .SomeCondition %}bar{% else %}baz{% end %} qux">
			Example Go template conditional inside a quoted attribute.
		</p>

		{% range .Items %}
			<p>Example Go template range: {% .ItemName %}</p>
		{% else %}
			<p>No items.</p>
		{% end %}
	<script src="/dist/index.js"></script></body>
</html>
`

func TestInsertAssets_WithNonce_Success(t *testing.T) {
	actualOutput, err := insertAssets(strings.NewReader(sampleHTML), jsPath, cssPath, nonce)
	require.NoError(t, err)
	assert.Equal(t, expectedHTMLWithNonce, actualOutput)
}

func TestInsertAssets_WithoutNonce_Success(t *testing.T) {
	actualOutput, err := insertAssets(strings.NewReader(sampleHTML), jsPath, cssPath, "" /* =nonce */)
	require.NoError(t, err)
	assert.Equal(t, expectedHTMLWithoutNonce, actualOutput)
}

func TestInsertAssets_MissingEndHeadTag_Error(t *testing.T) {
	html := `
	<html>
		<head>
			<title>Hello</title>
		<body>
			<p>Hello, world!</p>
		</body>
	</html>
	`
	_, err := insertAssets(strings.NewReader(html), jsPath, cssPath, "" /* =nonce */)
	assert.EqualError(t, err, "no </head> tag found")
}

func TestInsertAssets_MalformedEndHeadTag_Error(t *testing.T) {
	html := `
	<html>
		<head>
			<title>Hello</title>
		<head>
		<body>
			<p>Hello, world!</p>
		</body>
	</html>
	`
	_, err := insertAssets(strings.NewReader(html), jsPath, cssPath, "" /* =nonce */)
	assert.EqualError(t, err, "no </head> tag found")
}

func TestInsertAssets_MissingEndBodyTag_Error(t *testing.T) {
	html := `
	<html>
		<head>
			<title>Hello</title>
		</head>
		<body>
			<p>Hello, world!</p>
	</html>
	`
	_, err := insertAssets(strings.NewReader(html), jsPath, cssPath, "" /* =nonce */)
	assert.EqualError(t, err, "no </body> tag found")
}

func TestInsertAssets_MalformedEndBodyTag_Error(t *testing.T) {
	html := `
	<html>
		<head>
			<title>Hello</title>
		</head>
		<body>
			<p>Hello, world!</p>
		<body>
	</html>
	`
	_, err := insertAssets(strings.NewReader(html), jsPath, cssPath, "" /* =nonce */)
	assert.EqualError(t, err, "no </body> tag found")
}
