package ws

import "testing"

func TestDecide(t *testing.T) {
    cases := []struct{
        a, b string
        want string
    }{
        {"rock", "scissors", "win"},
        {"rock", "paper", "lose"},
        {"paper", "rock", "win"},
        {"scissors", "scissors", "draw"},
    }

    for _, tc := range cases {
        if got := decide(tc.a, tc.b); got != tc.want {
            t.Fatalf("decide(%s,%s) = %s; want %s", tc.a, tc.b, got, tc.want)
        }
    }
}
