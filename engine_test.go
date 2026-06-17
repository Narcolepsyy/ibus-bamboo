package main

import (
	"ibus-bamboo/config"
	"sync"
	"testing"

	"github.com/BambooEngine/bamboo-core"
)

type keyEvent struct {
	keys                [3]uint32
	canBeProcessed      bool
	expectedPreeditText string
	expectedCommitText  string
}

func asciiToKeys(s rune) [3]uint32 {
	return [3]uint32{uint32(s), uint32(s), 0}
}

func generateKeyEvents(s string, v []string, appendKeys ...keyEvent) []keyEvent {
	var kv []keyEvent
	for i, c := range s {
		kv = append(kv, keyEvent{keys: asciiToKeys(c), canBeProcessed: true, expectedPreeditText: v[i], expectedCommitText: v[i]})
	}
	return append(kv, appendKeys...)
}

func generateMetaKeyEvent(keys [3]uint32) func(expectedText ...string) keyEvent {
	return func(expectedText ...string) keyEvent {
		kv := keyEvent{keys: keys, canBeProcessed: false}
		if len(expectedText) > 0 {
			kv.expectedCommitText = expectedText[0]
		}
		if len(expectedText) > 1 {
			kv.expectedPreeditText = expectedText[1]
		}
		return kv
	}
}

var enter = generateMetaKeyEvent([3]uint32{0xff0d, 0xff0d, 0})
var control = generateMetaKeyEvent([3]uint32{0xffe3, 0xffe3, 0})

type testCase struct {
	name      string
	keyEvents []keyEvent
	inputMode int
	mTable    map[string]string
}

func TestPreeditEngine(t *testing.T) {
	for _, tc := range []testCase{
		{
			name: "empty_key_events",
			keyEvents: []keyEvent{
				{
					keys:           [3]uint32{0, 0, 0},
					canBeProcessed: false,
				},
			},
		},
		{
			name: "control_a",
			keyEvents: []keyEvent{
				control(),
				{keys: [3]uint32{0x0061, 0x0061, 4}, canBeProcessed: false, expectedPreeditText: ""},
			},
		},
		{
			name:   "macro_control_a",
			mTable: map[string]string{"->": "arrow"},
			keyEvents: []keyEvent{
				control(),
				{keys: [3]uint32{0x0061, 0x0061, 4}, canBeProcessed: false, expectedPreeditText: ""},
			},
		},
		{
			name:      "duowidro",
			keyEvents: generateKeyEvents("duowidro", []string{"d", "du", "duo", "dươ", "dươi", "đươi", "đưởi", "đuổi"}),
		},
		{
			name:      "duowidro_enter",
			keyEvents: generateKeyEvents("duowidro", []string{"d", "du", "duo", "dươ", "dươi", "đươi", "đưởi", "đuổi"}, enter("đuổi")),
		},
		{
			name:      "macro_vowl_space",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vowl ", []string{"v", "vo", "vơ", "vơl", ""}, control("vowl ")),
		},
		{
			name:      "macro_vowl_enter",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vowl", []string{"v", "vo", "vơ", "vơl"}, enter("vowl")),
		},
		{
			name:      "macro_duowidro_enter",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("duowidro", []string{"d", "du", "duo", "dươ", "dươi", "đươi", "đưởi", "đuổi"}, enter("đuổi")),
		},
		{
			name: "workaround_spreadsheet_number_enter",
			keyEvents: []keyEvent{
				{keys: asciiToKeys('1'), canBeProcessed: false, expectedPreeditText: ""},
				{keys: asciiToKeys('2'), canBeProcessed: false, expectedPreeditText: ""},
				enter(),
			},
		},
		{
			name:      "macro_vn_dot",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vn.", []string{"v", "vn", ""}, enter("việt nam.")),
		},
		{
			name:   "macro_vn_comma_space",
			mTable: map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vn", []string{"v", "vn"}, []keyEvent{
				{keys: asciiToKeys(','), canBeProcessed: true, expectedCommitText: "việt nam,"},
				{keys: asciiToKeys(' '), canBeProcessed: true, expectedCommitText: " "},
				enter("việt nam, "),
			}...),
		},
		{
			name:      "macro_vn_enter",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vn", []string{"v", "vn"}, enter("việt nam")),
		},
		{
			name:      "macro_arrow_dot",
			mTable:    map[string]string{"->": "arrow"},
			keyEvents: generateKeyEvents("->.", []string{"-", "->", ""}, control("arrow.")),
		},
		{
			name:      "macro_arrow_enter",
			mTable:    map[string]string{"->": "arrow"},
			keyEvents: generateKeyEvents("->", []string{"-", "->"}, enter("arrow")),
		},
		{
			name:   "macro_csao_space",
			mTable: map[string]string{"csao": "✪", "csao2": "✬"},
			keyEvents: generateKeyEvents("csao", []string{"c", "cs", "csa", "csao"}, []keyEvent{
				{keys: asciiToKeys(' '), canBeProcessed: true, expectedCommitText: "✪ "},
			}...),
		},
		{
			name:      "macro_csao2_enter",
			mTable:    map[string]string{"csao": "✪", "csao2": "✬"},
			keyEvents: generateKeyEvents("csao2", []string{"c", "cs", "csa", "csao", "csao2"}, enter("✬")),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputMode = config.PreeditIM
			assertEngine(t, tc, func(t testing.TB, fe *fakeEngine, e IEngine) {
				for _, ev := range tc.keyEvents {
					keys := ev.keys
					t.Logf("Processing key %c %v", rune(keys[0]), keys)
					ret, _ := e.ProcessKeyEvent(keys[0], keys[1], keys[2])
					if ret != ev.canBeProcessed {
						t.Errorf("Is key can be processed? expected (%v), got (%v).", ev.canBeProcessed, ret)
					}
					if ev.canBeProcessed && fe.preeditText != ev.expectedPreeditText {
						t.Errorf("Preedit text, expected (%s), got (%s).", ev.expectedPreeditText, fe.preeditText)
					}
					if !ev.canBeProcessed && ev.expectedCommitText != fe.commitText {
						t.Errorf("Commit text, expected (%s), got (%s).", ev.expectedCommitText, fe.commitText)
					}
				}
			})
		})
	}
}

func TestBsEngine(t *testing.T) {
	for _, tc := range []testCase{
		{
			name: "empty_key_events",
			keyEvents: []keyEvent{
				{
					keys:           [3]uint32{0, 0, 0},
					canBeProcessed: false,
				},
			},
		},
		{
			name: "control_a",
			keyEvents: []keyEvent{
				control(),
				{keys: [3]uint32{0x0061, 0x0061, 4}, canBeProcessed: false}, // Ctrl+A
			},
		},
		{
			name:      "vn_dot_enter",
			keyEvents: generateKeyEvents("vn.", []string{"v", "vn", "vn."}, enter("vn.")),
		},
		{
			name:   "macro_control_a",
			mTable: map[string]string{"->": "arrow"},
			keyEvents: []keyEvent{
				control(),
				{keys: [3]uint32{0x0061, 0x0061, 4}, canBeProcessed: false}, // Ctrl+A
			},
		},
		{
			name:      "duowidro",
			keyEvents: generateKeyEvents("duowidro", []string{"d", "du", "duo", "dươ", "dươi", "đươi", "đưởi", "đuổi"}),
		},
		{
			name:      "duowidro_enter",
			keyEvents: generateKeyEvents("duowidro", []string{"d", "du", "duo", "dươ", "dươi", "đươi", "đưởi", "đuổi"}, enter("đuổi")),
		},
		{
			name:      "macro_vowl_space",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vowl ", []string{"v", "vo", "vơ", "vơl", "vowl "}),
		},
		{
			name:      "macro_vowl_enter",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vowl", []string{"v", "vo", "vơ", "vơl"}, enter("vowl")),
		},
		{
			name:      "macro_duowidro_enter",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("duowidro", []string{"d", "du", "duo", "dươ", "dươi", "đươi", "đưởi", "đuổi"}, enter("đuổi")),
		},
		{
			name: "workaround_spreadsheet_number_enter",
			keyEvents: []keyEvent{
				{keys: asciiToKeys('1'), canBeProcessed: false, expectedPreeditText: ""},
				{keys: asciiToKeys('2'), canBeProcessed: false, expectedPreeditText: ""},
				enter(),
			},
		},
		{
			name:   "macro_12",
			mTable: map[string]string{"vn": "việt nam"},
			keyEvents: []keyEvent{
				{keys: asciiToKeys('1'), canBeProcessed: true, expectedCommitText: "1"},
				{keys: asciiToKeys('2'), canBeProcessed: true, expectedCommitText: "12"},
				enter("12"),
			},
		},
		{
			name:      "macro_vn_dot",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vn.", []string{"v", "vn", "việt nam."}),
		},
		{
			name:      "macro_vn_comma_space",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vn, ", []string{"v", "vn", "việt nam,", "việt nam, "}),
		},
		{
			name:      "macro_vn_enter",
			mTable:    map[string]string{"vn": "việt nam"},
			keyEvents: generateKeyEvents("vn", []string{"v", "vn"}, enter("việt nam")),
		},
		{
			name:      "macro_arrow_dot",
			mTable:    map[string]string{"->": "arrow"},
			keyEvents: generateKeyEvents("->.", []string{"-", "->", "arrow."}),
		},
		{
			name:      "macro_arrow_enter",
			mTable:    map[string]string{"->": "arrow"},
			keyEvents: generateKeyEvents("->", []string{"-", "->"}, enter("arrow")),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.inputMode = config.SurroundingTextIM
			assertEngine(t, tc, func(t testing.TB, fe *fakeEngine, e IEngine) {
				for _, ev := range tc.keyEvents {
					keys := ev.keys
					t.Logf("Processing key %c %v", rune(keys[0]), keys)
					ret, _ := e.ProcessKeyEvent(keys[0], keys[1], keys[2])
					if ret != ev.canBeProcessed {
						t.Errorf("Is key can be processed? expected (%v), got (%v).", ev.canBeProcessed, ret)
					}
					if fe.commitText != ev.expectedCommitText {
						t.Errorf("Commit text, expected (%s), got (%s).", ev.expectedCommitText, fe.commitText)
					}
				}
			})
		})
	}
}

func assertEngine(t testing.TB, tc testCase, assertFn func(testing.TB, *fakeEngine, IEngine)) {
	fe := NewFakeEngine()
	engineName := "test"
	var cfg = config.DefaultCfg()
	cfg.DefaultInputMode = tc.inputMode
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	if tc.mTable != nil {
		cfg.IBflags |= config.IBmacroEnabled
	}
	e := NewIbusBambooEngine(engineName, &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))
	if tc.mTable != nil {
		e.macroTable = &MacroTable{
			mTable: tc.mTable,
		}
	}
	assertFn(t, fe, e)
}

func TestResetClearsBufferInPreeditIM(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.PreeditIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	for _, ch := range "tooi" {
		e.preeditor.ProcessKey(ch, bamboo.VietnameseMode)
	}
	if e.getRawKeyLen() == 0 {
		t.Fatal("Buffer should not be empty before Reset")
	}

	e.Reset()

	if e.getRawKeyLen() != 0 {
		t.Errorf("After Reset() in PreeditIM, rawKeyLen should be 0, got %d", e.getRawKeyLen())
	}
}

func TestResetPreservesBufferInBackspaceModes(t *testing.T) {
	for _, inputMode := range []int{config.SurroundingTextIM, config.BackspaceForwardingIM, config.ForwardAsCommitIM, config.XTestFakeKeyEventIM} {
		t.Run(config.ImLookupTable[inputMode], func(t *testing.T) {
			fe := NewFakeEngine()
			cfg := config.DefaultCfg()
			cfg.DefaultInputMode = inputMode
			inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
			e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

			for _, ch := range "tooi" {
				e.preeditor.ProcessKey(ch, bamboo.VietnameseMode)
			}
			if e.getRawKeyLen() == 0 {
				t.Fatal("Buffer should not be empty before Reset")
			}

			// In backspace modes, Reset() must NOT clear the buffer
			// because Chrome/apps call Reset() aggressively between
			// keystrokes — clearing would break multi-keystroke words.
			e.Reset()

			if e.getRawKeyLen() == 0 {
				t.Errorf("After Reset() in %s, buffer should be preserved", config.ImLookupTable[inputMode])
			}
		})
	}
}

func TestResetDrainsKeyPressChan(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.SurroundingTextIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	// Enqueue some fake keystrokes
	keyPressChan <- [3]uint32{uint32('a'), uint32('a'), 0}
	keyPressChan <- [3]uint32{uint32('b'), uint32('b'), 0}

	if len(keyPressChan) != 2 {
		t.Fatalf("Expected 2 queued keys, got %d", len(keyPressChan))
	}

	e.Reset()

	if len(keyPressChan) != 0 {
		t.Errorf("After Reset(), keyPressChan should be drained, got %d remaining", len(keyPressChan))
	}
}

func TestFocusInPreservesBufferInBackspaceMode(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.SurroundingTextIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	// First FocusIn sets wmClasses to the current WM class
	e.FocusIn()

	// Type some Vietnamese text
	for _, ch := range "tooi" {
		e.preeditor.ProcessKey(ch, bamboo.VietnameseMode)
	}
	if e.getRawKeyLen() == 0 {
		t.Fatal("Buffer should not be empty before second FocusIn")
	}

	// Second FocusIn with same WM class should NOT reset buffer
	// (apps fire FocusIn mid-typing for tooltips, autocomplete, etc.)
	e.FocusIn()

	if e.getRawKeyLen() == 0 {
		t.Errorf("After FocusIn() in SurroundingTextIM with same WM class, buffer should be preserved")
	}
}

func TestFocusOutResetsBufferInPreeditIM(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.PreeditIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	for _, ch := range "tooi" {
		e.preeditor.ProcessKey(ch, bamboo.VietnameseMode)
	}
	if e.getRawKeyLen() == 0 {
		t.Fatal("Buffer should not be empty before FocusOut")
	}

	e.FocusOut()

	if e.getRawKeyLen() != 0 {
		t.Errorf("After FocusOut() in PreeditIM, buffer should be cleared, got rawKeyLen=%d", e.getRawKeyLen())
	}
}

func TestFocusOutPreservesBufferInBackspaceMode(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.SurroundingTextIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	for _, ch := range "tooi" {
		e.preeditor.ProcessKey(ch, bamboo.VietnameseMode)
	}
	if e.getRawKeyLen() == 0 {
		t.Fatal("Buffer should not be empty before FocusOut")
	}

	e.FocusOut()

	if e.getRawKeyLen() == 0 {
		t.Errorf("After FocusOut() in SurroundingTextIM, buffer should be preserved")
	}
}

func TestFakeBackspaceResetAfterSendCycle(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.SurroundingTextIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	// Manually set a non-zero fake backspace counter
	e.setFakeBackspace(5)
	if e.getFakeBackspace() != 5 {
		t.Fatalf("Expected fakeBackspace=5, got %d", e.getFakeBackspace())
	}

	// Reset should clear it
	e.resetFakeBackspace()
	if e.getFakeBackspace() != 0 {
		t.Errorf("After resetFakeBackspace(), expected 0, got %d", e.getFakeBackspace())
	}
}

func TestGetOffsetRunes(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.SurroundingTextIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	tests := []struct {
		name      string
		newText   string
		oldText   string
		wantRunes string
		wantNBS   int
	}{
		{"same_text", "tôi", "tôi", "", 0},
		{"append_char", "tôi ", "tôi", " ", 0},
		{"replace_middle", "tối", "tôi", "ối", 2},
		{"empty_old", "abc", "", "abc", 0},
		{"empty_new", "", "abc", "", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRunes, gotNBS := e.getOffsetRunes(tt.newText, tt.oldText)
			if string(gotRunes) != tt.wantRunes {
				t.Errorf("getOffsetRunes(%q, %q) runes = %q, want %q", tt.newText, tt.oldText, string(gotRunes), tt.wantRunes)
			}
			if gotNBS != tt.wantNBS {
				t.Errorf("getOffsetRunes(%q, %q) nBS = %d, want %d", tt.newText, tt.oldText, gotNBS, tt.wantNBS)
			}
		})
	}
}

// TestConcurrentResetAndKeyPress exercises the data race between
// Reset/FocusIn/FocusOut (D-Bus thread) and keyPressHandler (worker goroutine).
// Run with: go test -race -run TestConcurrentResetAndKeyPress
func TestConcurrentResetAndKeyPress(t *testing.T) {
	fe := NewFakeEngine()
	cfg := config.DefaultCfg()
	cfg.DefaultInputMode = config.SurroundingTextIM
	inputMethod := bamboo.ParseInputMethod(cfg.InputMethodDefinitions, cfg.InputMethod)
	e := NewIbusBambooEngine("test", &cfg, fe, bamboo.NewEngine(inputMethod, cfg.Flags))

	const iterations = 200
	var wg sync.WaitGroup

	// Goroutine 1: simulate keyPressHandler mutations (worker thread)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			e.Lock()
			e.preeditor.ProcessKey('a', bamboo.VietnameseMode)
			_ = e.getPreeditString()
			e.preeditor.Reset()
			e.Unlock()
		}
	}()

	// Goroutine 2: simulate Reset/FocusIn/FocusOut (D-Bus thread)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			switch i % 3 {
			case 0:
				e.Reset()
			case 1:
				e.FocusIn()
			case 2:
				e.FocusOut()
			}
		}
	}()

	wg.Wait()
	// If we get here without a race detector complaint, the lock is working.
}
