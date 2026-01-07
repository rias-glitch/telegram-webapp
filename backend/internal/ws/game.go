package ws

// decide определяет результат игрока A против игрока B
// moveA / moveB: "rock" | "paper" | "scissors"
func decide(moveA, moveB string) string {
	if moveA == moveB {
		return "draw"
	}

	switch moveA {
	case "rock":
		if moveB == "scissors" {
			return "win"
		}
	case "paper":
		if moveB == "rock" {
			return "win"
		}
	case "scissors":
		if moveB == "paper" {
			return "win"
		}
	}

	return "lose"
}
