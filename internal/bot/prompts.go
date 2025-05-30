package bot

import (
	"bytes"
	_ "embed" // Embed templates
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"text/template"

	"github.com/AlexGustafsson/clabbe/internal/llm"
)

//go:embed prompt-song-suggestion.tmpl
var defaultSongSuggestionTemplate string

//go:embed prompt-theme-suggestion.tmpl
var defaultThemeSuggestionTemplate string

func NewThemeRequest() *llm.ChatRequest {
	prompt := `Respond with themes for songs to include in a playlist. Respond with five themes, one theme per line, each containing at least four words. Don't include seasonal themes such as summer vibes or Christmas songs.
Don't include the following words or similar words:
Workout
Empowerment
Feel-good
Study
Love`

	examples := strings.Split(`Underground indie rock vibes
Classic rock ballads revisited
West coast hip-hop classics
Latin jazz fusion grooves
Acoustic soulful folk tunes
Underground indie rock vibes
Classic rock ballads revisited
West coast hip-hop classics
Latin jazz fusion grooves
Acoustic soulful folk tunes
Synthwave retro cyber beats
Reggae essentials for relaxation
Indie pop anthems for road trips
90s R&B slow jams sentiment
High-energy EDM bangers
Chill lo-fi hip-hop vibes
Alternative rock anthems mix
Country music storytelling tunes
Jazzy swing dance classics`, "\n")
	// Pick a few examples to include for context, providing a bit more random
	// state to the model, which is otherwise quite deterministic
	selectedExamples := make([]string, 5)
	for i := 0; i < 5; i++ {
		selectedExamples[i] = examples[rand.IntN(len(examples))]
	}

	return &llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: prompt,
			},
			{
				Role:    llm.RoleAssistant,
				Content: strings.Join(selectedExamples, "\n"),
			},
		},
	}
}

func RenderPrompt(text string, data any) (string, error) {
	x := template.New("")

	x.Funcs(template.FuncMap{
		"pick":   pick,
		"split":  split,
		"render": withRender(x),
		"first":  first,
	})

	_, err := x.Parse(text)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	if err := x.Execute(&buffer, data); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func pick(arg0 reflect.Value, arg1 reflect.Value) (reflect.Value, error) {
	arg0 = indirectInterface(arg0)
	if !arg0.IsValid() {
		return reflect.Value{}, fmt.Errorf("pick on untyped nil")
	}

	switch arg0.Kind() {
	case reflect.String, reflect.Array, reflect.Slice:
		// OK
	default:
		return reflect.Value{}, fmt.Errorf("can't pick item from type %s", arg0.Type())
	}

	n, err := indexArg(arg1, arg0.Cap())
	if err != nil {
		return reflect.Value{}, err
	}

	var result reflect.Value
	switch arg0.Kind() {
	case reflect.String:
		result = reflect.MakeSlice(reflect.SliceOf(arg0.Type()), min(arg0.Len(), n), min(arg0.Len(), n))
	case reflect.Array, reflect.Slice:
		result = reflect.MakeSlice(reflect.SliceOf(arg0.Type().Elem()), min(arg0.Len(), n), min(arg0.Len(), n))
	default:
		return reflect.Value{}, fmt.Errorf("can't pick item from type %s", arg0.Type())
	}

	// TODO: Optimize?
	indexes := rand.Perm(arg0.Len())
	for i := 0; i < n && i < len(indexes); i++ {
		result.Index(i).Set(arg0.Index(indexes[i]))
	}

	return result, nil
}

func first(arg0 reflect.Value, arg1 reflect.Value) (reflect.Value, error) {
	arg0 = indirectInterface(arg0)
	if !arg0.IsValid() {
		return reflect.Value{}, fmt.Errorf("pick first on untyped nil")
	}

	switch arg0.Kind() {
	case reflect.String, reflect.Array, reflect.Slice:
		// OK
	default:
		return reflect.Value{}, fmt.Errorf("can't pick first items from type %s", arg0.Type())
	}

	n, err := indexArg(arg1, arg0.Cap())
	if err != nil {
		return reflect.Value{}, err
	}

	var result reflect.Value
	switch arg0.Kind() {
	case reflect.String:
		result = reflect.MakeSlice(reflect.SliceOf(arg0.Type()), min(arg0.Len(), n), min(arg0.Len(), n))
	case reflect.Array, reflect.Slice:
		result = reflect.MakeSlice(reflect.SliceOf(arg0.Type().Elem()), min(arg0.Len(), n), min(arg0.Len(), n))
	default:
		return reflect.Value{}, fmt.Errorf("can't pick first items from type %s", arg0.Type())
	}

	for i := 0; i < n && i < arg0.Len(); i++ {
		result.Index(i).Set(arg0.Index(i))
	}

	return result, nil
}

func split(arg0 reflect.Value, arg1 reflect.Value) (reflect.Value, error) {
	var isNil bool
	arg0, isNil = indirect(arg0)
	if isNil {
		return reflect.Value{}, fmt.Errorf("can't split nil value")
	}

	arg1, isNil = indirect(arg1)
	if isNil {
		return reflect.Value{}, fmt.Errorf("can't split using nil value")
	}

	if arg0.Kind() != reflect.String {
		return reflect.Value{}, fmt.Errorf("can't split non-string of type %s", arg0.Type())
	}

	if arg0.Kind() != reflect.String {
		return reflect.Value{}, fmt.Errorf("can't split using non-string of type %s", arg0.Type())
	}

	result := strings.Split(arg0.String(), arg1.String())
	return reflect.ValueOf(result), nil
}

func withRender(template *template.Template) any {
	return func(arg0 reflect.Value) (reflect.Value, error) {
		var isNil bool
		arg0, isNil = indirect(arg0)
		if isNil {
			return reflect.Value{}, fmt.Errorf("can't render nil value")
		}

		if arg0.Kind() != reflect.String {
			return reflect.Value{}, fmt.Errorf("can't use non-string of type %s as template name", arg0.Type())
		}

		var buffer bytes.Buffer
		if err := template.ExecuteTemplate(&buffer, arg0.String(), nil); err != nil {
			return reflect.Value{}, nil
		}

		return reflect.ValueOf(buffer.String()), nil
	}
}

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Interface {
		return v
	}
	if v.IsNil() {
		return reflect.Value{}
	}
	return v.Elem()
}

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func indexArg(index reflect.Value, cap int) (int, error) {
	var x int64
	switch index.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x = index.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x = int64(index.Uint())
	case reflect.Invalid:
		return 0, fmt.Errorf("cannot index slice/array with nil")
	default:
		return 0, fmt.Errorf("cannot index slice/array with type %s", index.Type())
	}
	if x < 0 || int(x) < 0 || int(x) > cap {
		return 0, fmt.Errorf("index out of range: %d", x)
	}
	return int(x), nil
}
