// HOF skill schemas for GF(3) balanced composition
package boxxy

#Trit: 0 | 1 | 2

#Skill: {
	name: string
	trit: #Trit
	role: "PLUS" | "ERGODIC" | "MINUS"
}

#Triad: {
	a: #Skill
	b: #Skill
	c: #Skill
	_bal: (a.trit + b.trit + c.trit) % 3 & 0
}

#BetaTrace: {
	term:   string
	steps:  [...{bind: string, result: string}]
	normal: string
}
