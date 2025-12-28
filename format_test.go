package main

import (
	"html/template"
	"testing"
)

func TestFormatComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected template.HTML
	}{
		{
			name:     "Basic text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Greentext",
			input:    ">be me\n>coding",
			expected: "<span class=\"greentext\">&gt;be me</span><br><span class=\"greentext\">&gt;coding</span>",
		},
		{
			name:     "Reply link",
			input:    ">>123",
			expected: "<a class=\"reply-link\" href=\"#p123\">&gt;&gt;123</a>",
		},
		{
			name:     "Mixed formatting",
			input:    ">quote\n>>456\nNormal text",
			expected: "<span class=\"greentext\">&gt;quote</span><br><a class=\"reply-link\" href=\"#p456\">&gt;&gt;456</a><br>Normal text",
		},
		{
			name:     "XSS Prevention",
			input:    "<script>alert(1)</script>",
			expected: "&lt;script&gt;alert(1)&lt;/script&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatComment(tt.input)
			if got != tt.expected {
				t.Errorf("formatComment() = %v, want %v", got, tt.expected)
			}
		})
	}
}
