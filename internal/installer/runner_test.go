package installer_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"context"

	"github.com/epinio/installer/internal/installer"
)

type spy struct {
	Visited map[string]bool
}

var _ installer.Action = &spy{}

var lock = &sync.RWMutex{}

func (s spy) Apply(ctx context.Context, c installer.Component) error {
	lock.Lock()
	defer lock.Unlock()
	s.Visited[c.String()] = true
	return nil
}

type errspy struct {
	Visited map[string]bool
}

var _ installer.Action = &spy{}

func (e errspy) Apply(ctx context.Context, c installer.Component) error {
	lock.Lock()
	e.Visited[c.String()] = true
	lock.Unlock()

	time.Sleep(1 * time.Second)
	if c.String() == "linkerd" {
		return errors.New("error on first component")
	}

	return nil
}

var _ = Describe("Runner", func() {
	Describe("Walk", func() {
		var m *installer.Manifest

		BeforeEach(func() {
			var err error
			m, err = installer.Load(assetPath("test-manifest.yml"))
			Expect(err).ToNot(HaveOccurred())

		})

		It("visits all components", func() {
			s := &spy{Visited: map[string]bool{}}
			err := installer.Walk(context.TODO(), m.Components, s)
			Expect(err).ToNot(HaveOccurred())
			for _, v := range s.Visited {
				Expect(v).To(BeTrue())
			}
		})

		It("does not start new actions after err", func() {
			// should check it doesn't cancel running actions
			s := &errspy{Visited: map[string]bool{}}
			err := installer.Walk(context.TODO(), m.Components, s)
			Expect(err).To(HaveOccurred())
			Expect(s.Visited).To(HaveLen(2))
			Expect(s.Visited).To(HaveKeyWithValue("epinio-namespace", true))
			Expect(s.Visited).To(HaveKeyWithValue("linkerd", true))
		})
	})
})
