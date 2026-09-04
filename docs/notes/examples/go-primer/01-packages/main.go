// Every Go file starts by declaring which "package" it belongs to.
// "main" is special: it's the one that produces a runnable program.
package main

// The standard library is organised into packages. You import the ones you use.
// "fmt" = formatting (print/scan). Like Python's built-in print, but you import it.
import "fmt"

// Execution starts here. A program with package main MUST have exactly one func main.
// (Bash: the top of the script. Python: the "if __name__ == '__main__'" block.)
func main() {
	fmt.Println("hello from the fluke-operator go primer")
}
