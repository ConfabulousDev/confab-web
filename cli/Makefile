.PHONY: build clean test

build:
	go build -o confab

clean:
	rm -f confab

test:
	cat test_hook_with_agents.json | ./confab save
