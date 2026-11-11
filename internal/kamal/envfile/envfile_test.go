package envfile

import "testing"

func TestRender(t *testing.T) {
	if got := Render([]Var{{"foo", "bar"}, {"baz", "haz"}}); got != "foo=bar\nbaz=haz\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderWontEscapeHash(t *testing.T) {
	if got := Render([]Var{{"foo", "#$foo"}, {"bar", "#{bar}"}}); got != "foo=#$foo\nbar=#{bar}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderWontEscapeNonASCII(t *testing.T) {
	cases := []string{
		`你好 means hello, "欢迎" means welcome, that's simple! 😃 {smile}`,
		`こんにちは means hello, "ようこそ" means welcome, that's simple! 😃 {smile}`,
		`안녕하세요 means hello, "어서 오십시오" means welcome, that's simple! 😃 {smile}`,
	}
	for _, value := range cases {
		if got := Render([]Var{{"foo", value}}); got != "foo="+value+"\n" {
			t.Errorf("got %q", got)
		}
	}
}

func TestRenderEmpty(t *testing.T) {
	if got := Render(nil); got != "\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderEscapedNewline(t *testing.T) {
	if got := Render([]Var{{"foo", `hello\nthere`}}); got != "foo=hello\\\\nthere\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderNewline(t *testing.T) {
	if got := Render([]Var{{"foo", "hello\nthere"}}); got != "foo=hello\\nthere\n" {
		t.Fatalf("got %q", got)
	}
}
