package installer

import (
	"context"
	"fmt"
	"os"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Action interface {
	Apply(context.Context, Component) error
}

// WalkSerially walks through the nodes, applies Action and waits for it to finish.
// Walks nodes in sequencially (as opposed to "Walk").
func WalkSerially(ctx context.Context, plan Components, action Action) error {
	for _, c := range plan {
		err := action.Apply(ctx, c)
		if err != nil {
			fmt.Printf("error for '%s': %v\n", c.ID, err)
			return err
		}
	}
	return nil
}

// Walk all the nodes, apply Action and wait for it to finish. Walk nodes in parallel, if parents ("needs") are done.
func Walk(ctx context.Context, plan Components, action Action) error {
	// access to these vars from the goroutines will need to be
	// synchronized by RWMutex to avoid races
	done := map[DeploymentID]bool{}
	running := map[DeploymentID]bool{}
	for _, c := range plan {
		done[c.ID] = false
		running[c.ID] = false
	}
	noErr := true

	g := new(errgroup.Group)
	lock := &sync.RWMutex{}
	processMore := func() bool {
		lock.RLock()
		defer lock.RUnlock()
		return noErr
	}
	for !allDone(lock, done) && processMore() {
		for _, c := range plan {
			// avoid pointer capture
			c := c
			lock.RLock()
			if done[c.ID] {
				//fmt.Printf("skip done: %s\n", c.ID)
				lock.RUnlock()
				continue
			}
			if running[c.ID] {
				//fmt.Printf("skip running: %s\n", c.ID)
				lock.RUnlock()
				continue
			}
			if c.Needs != "" && !done[c.Needs] {
				//fmt.Printf("skip '%s' for deps: %s (r:%v, d:%v)\n", c.ID, c.Needs, running[c.Needs], done[c.Needs])
				lock.RUnlock()
				continue
			}
			lock.RUnlock()

			lock.Lock()
			running[c.ID] = true
			lock.Unlock()

			g.Go(func() error {
				err := action.Apply(ctx, c)
				if err != nil {
					fmt.Printf("error for '%s': %v\n", c.ID, err)
					// don't start any more tasks, break the for loop
					lock.Lock()
					noErr = false
					lock.Unlock()
					return err
				}

				lock.Lock()
				done[c.ID] = true
				lock.Unlock()

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		fmt.Println("failed to install all components")
		return err
	}

	return nil
}

// ReverseWalk all the nodes, apply Action and wait for it to finish. Walk nodes in parallel, blocks if node still has running needers
func ReverseWalk(ctx context.Context, plan Components, action Action) {
	done := map[DeploymentID]bool{}
	running := map[DeploymentID]bool{}
	for _, c := range plan {
		done[c.ID] = false
		running[c.ID] = false
	}

	needers := map[DeploymentID][]DeploymentID{}
	for _, c := range plan {
		if c.Needs != "" {
			needers[c.Needs] = append(needers[c.Needs], c.ID)
		}
	}

	wg := &sync.WaitGroup{}
	var lock = &sync.RWMutex{}
	for !allDone(lock, done) {
		for _, c := range plan {
			c := c
			lock.RLock()
			if done[c.ID] {
				lock.RUnlock()
				continue
			}
			if running[c.ID] {
				lock.RUnlock()
				continue
			}
			if len(needers[c.ID]) > 0 {
				blocked := false
				for _, n := range needers[c.ID] {
					if !done[n] {
						blocked = true
						break
					}

				}
				if blocked {
					lock.RUnlock()
					continue
				}
			}
			lock.RUnlock()

			lock.Lock()
			running[c.ID] = true
			lock.Unlock()

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()

				if err := action.Apply(ctx, c); err != nil {
					fmt.Println(err)
					os.Exit(-1)
				}

				lock.Lock()
				done[c.ID] = true
				lock.Unlock()
			}(wg)
		}
	}

	wg.Wait()
}

func allDone(lock *sync.RWMutex, s map[DeploymentID]bool) bool {
	lock.RLock()
	defer lock.RUnlock()
	for _, done := range s {
		if !done {
			return false
		}
	}
	return true
}
