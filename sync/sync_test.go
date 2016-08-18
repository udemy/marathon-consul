package sync

import (
	"testing"

	"github.com/allegro/marathon-consul/consul"
	"github.com/allegro/marathon-consul/marathon"
	. "github.com/allegro/marathon-consul/utils"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"

	"fmt"
	"os"
	"time"

	"github.com/allegro/marathon-consul/apps"
	"github.com/allegro/marathon-consul/service"
)

func TestSyncJob_ShouldSyncOnLeadership(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 1)
	marathon := marathon.MarathonerStubWithLeaderForApps("current.leader:8080", app)
	services := newConsulServicesMock()
	sync := New(Config{
		Enabled:  true,
		Interval: 10 * time.Millisecond,
		Leader:   "current.leader:8080",
	}, marathon, services, noopSyncStartedListener)

	// when
	ticker := sync.StartSyncServicesJob()

	// then
	select {
	case <-time.After(15 * time.Millisecond):
		ticker.Stop()
		assert.Equal(t, 2, services.RegistrationsCount(app.Tasks[0].ID.String()))
	}
}

func TestSyncJob_ShouldNotSyncWhenDisabled(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 1)
	marathon := marathon.MarathonerStubWithLeaderForApps("current.leader:8080", app)
	services := newConsulServicesMock()
	sync := New(Config{
		Enabled:  false,
		Interval: 10 * time.Millisecond,
		Leader:   "current.leader:8080",
	}, marathon, services, noopSyncStartedListener)

	// when
	ticker := sync.StartSyncServicesJob()

	// then
	assert.Nil(t, ticker)
	select {
	case <-time.After(15 * time.Millisecond):
		assert.Equal(t, 0, services.RegistrationsCount(app.Tasks[0].ID.String()))
	}
}

func TestSyncJob_ShouldDefaultLeaderConfigurationToResolvedHostname(t *testing.T) {
	t.Parallel()
	// given
	hostname, _ := os.Hostname()
	app := ConsulApp("app1", 1)
	marathon := marathon.MarathonerStubWithLeaderForApps(fmt.Sprintf("%s:8080", hostname), app)
	services := newConsulServicesMock()
	sync := New(Config{
		Enabled:  true,
		Interval: 10 * time.Millisecond,
	}, marathon, services, noopSyncStartedListener)

	// when
	ticker := sync.StartSyncServicesJob()

	// then
	select {
	case <-time.After(15 * time.Millisecond):
		ticker.Stop()
		assert.Equal(t, 2, services.RegistrationsCount(app.Tasks[0].ID.String()))
	}
}

func TestSyncServices_ShouldNotSyncOnNoForceNorLeaderSpecified(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 1)
	marathon := marathon.MarathonerStubWithLeaderForApps("localhost:8080", app)
	services := newConsulServicesMock()
	sync := New(Config{}, marathon, services, noopSyncStartedListener)

	// when
	ticker := sync.StartSyncServicesJob()

	// then
	assert.Nil(t, ticker)
	assert.Zero(t, services.RegistrationsCount(app.Tasks[0].ID.String()))
}

func TestSyncServices_ShouldNotSyncOnNoLeadership(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 1)
	marathon := marathon.MarathonerStubWithLeaderForApps("leader:8080", app)
	services := newConsulServicesMock()
	sync := New(Config{Leader: "different.node:8090"}, marathon, services, noopSyncStartedListener)

	// when
	err := sync.SyncServices()

	// then
	assert.NoError(t, err)
	assert.Zero(t, services.RegistrationsCount(app.Tasks[0].ID.String()))
}

func TestSyncServices_ShouldSyncOnForceWithoutLeadership(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 1)
	marathon := marathon.MarathonerStubWithLeaderForApps("leader:8080", app)
	services := newConsulServicesMock()
	sync := New(Config{Leader: "different.node:8090", Force: true}, marathon, services, noopSyncStartedListener)

	// when
	err := sync.SyncServices()

	// then
	assert.NoError(t, err)
	assert.Equal(t, 1, services.RegistrationsCount(app.Tasks[0].ID.String()))
}

type ConsulServicesMock struct {
	registrations map[string]int
}

func newConsulServicesMock() *ConsulServicesMock {
	return &ConsulServicesMock{
		registrations: make(map[string]int),
	}
}

func (c *ConsulServicesMock) GetServices(name string) ([]*service.Service, error) {
	return nil, nil
}

func (c *ConsulServicesMock) GetAllServices() ([]*service.Service, error) {
	return nil, nil
}

func (c *ConsulServicesMock) Register(task *apps.Task, app *apps.App) error {
	c.registrations[task.ID.String()]++
	return nil
}

func (c *ConsulServicesMock) RegistrationsCount(instanceId string) int {
	return c.registrations[instanceId]
}

func (c *ConsulServicesMock) ServiceName(app *apps.App) string {
	return ""
}

func (c *ConsulServicesMock) DeregisterByTask(taskId apps.TaskId, agent string) error {
	return nil
}

func (c *ConsulServicesMock) Deregister(toDeregister *service.Service) error {
	return nil
}

func (c *ConsulServicesMock) GetAgent(agentAddress string) (*consulapi.Client, error) {
	return nil, nil
}

func (s *ConsulServicesMock) ServiceTaskId(service *service.Service) (apps.TaskId, error) {
	return apps.TaskId(""), nil
}

func TestSyncAppsFromMarathonToConsul(t *testing.T) {
	t.Parallel()
	// given
	marathoner := marathon.MarathonerStubForApps(
		ConsulApp("app1", 2),
		ConsulApp("app2", 1),
		NonConsulApp("app3", 1),
	)

	consul := consul.NewConsulStub()
	marathonSync := newSyncWithDefaultConfig(marathoner, consul)

	// when
	marathonSync.SyncServices()

	// then
	services, _ := consul.GetAllServices()
	assert.Equal(t, 3, len(services))
	for _, s := range services {
		assert.NotEqual(t, "app3", s.Name)
	}
}

func TestSyncAppsFromMarathonToConsul_CustomServiceName(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 3)
	app.Labels["consul"] = "customName"
	marathoner := marathon.MarathonerStubForApps(app)

	consul := consul.NewConsulStub()
	marathonSync := newSyncWithDefaultConfig(marathoner, consul)

	// when
	marathonSync.SyncServices()

	// then
	services, _ := consul.GetAllServices()
	assert.Equal(t, 3, len(services))
	assert.Equal(t, "customName", services[0].Name)
}

func TestRemoveInvalidServicesFromConsul(t *testing.T) {
	t.Parallel()
	// given
	marathoner := marathon.MarathonerStubForApps(
		ConsulApp("app1-invalid", 1),
		ConsulApp("app2", 1),
	)
	consul := consul.NewConsulStub()
	marathonSync := newSyncWithDefaultConfig(marathoner, consul)
	marathonSync.SyncServices()

	// when
	marathoner = marathon.MarathonerStubForApps(
		ConsulApp("app2", 1),
	)
	marathonSync = newSyncWithDefaultConfig(marathoner, consul)
	marathonSync.SyncServices()

	// then
	services, _ := consul.GetAllServices()
	assert.Equal(t, 1, len(services))
	assert.Equal(t, "app2", services[0].Name)
}

func TestRemoveInvalidServicesFromConsul_WithCustomServiceName(t *testing.T) {
	t.Parallel()
	// given
	invalidApp := ConsulApp("app1-invalid", 1)
	invalidApp.Labels["consul"] = "customName"
	marathoner := marathon.MarathonerStubForApps(
		invalidApp,
		ConsulApp("app2", 1),
	)
	consul := consul.NewConsulStub()
	marathonSync := newSyncWithDefaultConfig(marathoner, consul)

	// when
	marathonSync.SyncServices()

	// then
	customNameServices, _ := consul.GetServices("customName")
	assert.Equal(t, 1, len(customNameServices))

	// when
	marathoner = marathon.MarathonerStubForApps(
		ConsulApp("app2", 1),
	)
	marathonSync = newSyncWithDefaultConfig(marathoner, consul)
	marathonSync.SyncServices()

	// then
	customNameServices, _ = consul.GetServices("customName")
	assert.Equal(t, 0, len(customNameServices))

	services, _ := consul.GetAllServices()
	assert.Equal(t, 1, len(services))
	assert.Equal(t, "app2", services[0].Name)
}

func TestSyncOnlyHealthyServices(t *testing.T) {
	t.Parallel()
	// given
	marathoner := marathon.MarathonerStubForApps(
		ConsulApp("app1", 1),
		ConsulAppWithUnhealthyInstances("app2-one-unhealthy", 2, 1),
		ConsulAppWithUnhealthyInstances("app3-all-unhealthy", 2, 2),
	)
	consul := consul.NewConsulStub()
	marathonSync := newSyncWithDefaultConfig(marathoner, consul)

	// when
	marathonSync.SyncServices()

	// then
	services, _ := consul.GetAllServices()
	assert.Equal(t, 2, len(services))
	for _, s := range services {
		assert.NotEqual(t, "app3-all-unhealthy", s.Name)
	}
}

func TestSync_SkipServicesWithoutMarathonTaskTag(t *testing.T) {
	t.Parallel()
	// given
	app := ConsulApp("app1", 1)
	consul := consul.NewConsulStub()
	consul.RegisterWithoutMarathonTaskTag(&app.Tasks[0], app)
	marathonSync := newSyncWithDefaultConfig(marathon.MarathonerStubForApps(), consul)

	// when
	marathonSync.SyncServices()

	// then
	services, _ := consul.GetAllServices()
	assert.Equal(t, 1, len(services))
	assert.NotContains(t, services[0].Tags, service.MarathonTaskTag(app.Tasks[0].ID))
}

func TestSync_WithRegisteringProblems(t *testing.T) {
	t.Parallel()
	// given
	marathon := marathon.MarathonerStubForApps(ConsulApp("/test/app", 3))
	consul := consul.NewConsulStub()
	consul.FailRegisterForId("test_app.1")
	sync := newSyncWithDefaultConfig(marathon, consul)
	// when
	err := sync.SyncServices()
	services, _ := consul.GetAllServices()
	// then
	assert.NoError(t, err)
	assert.Len(t, services, 2)
}

func TestSync_WithDeregisteringProblems(t *testing.T) {
	t.Parallel()
	// given
	marathon := marathon.MarathonerStubForApps()
	consulStub := consul.NewConsulStub()
	notMarathonApp := ConsulApp("/not/marathon", 1)
	for _, task := range notMarathonApp.Tasks {
		consulStub.Register(&task, notMarathonApp)
	}
	allServices, _ := consulStub.GetAllServices()
	for _, s := range allServices {
		consulStub.FailDeregisterForId(s.ID)
	}

	sync := newSyncWithDefaultConfig(marathon, consulStub)
	// when
	err := sync.SyncServices()
	services, _ := consulStub.GetAllServices()
	// then
	assert.NoError(t, err)
	assert.Len(t, services, 1)
}

func TestSync_WithMarathonProblems(t *testing.T) {
	t.Parallel()
	// given
	marathon := errorMarathon{}
	sync := newSyncWithDefaultConfig(marathon, nil)
	// when
	err := sync.SyncServices()
	// then
	assert.Error(t, err)
}

func TestSync_WithConsulProblems(t *testing.T) {
	t.Parallel()
	// given
	marathon := marathon.MarathonerStubForApps(ConsulApp("/test/app", 3))
	serviceRegistry := errorServiceRegistry{}
	sync := newSyncWithDefaultConfig(marathon, serviceRegistry)
	// when
	err := sync.SyncServices()
	// then
	assert.Error(t, err)
}

func newSyncWithDefaultConfig(marathon marathon.Marathoner, serviceRegistry service.ServiceRegistry) *Sync {
	return New(Config{Enabled: true, Leader: "localhost:8080"}, marathon, serviceRegistry, noopSyncStartedListener)
}

func TestSync_AddingAgentsFromMarathonTasks(t *testing.T) {
	t.Parallel()

	consulServer := consul.CreateConsulTestServer(t)
	defer consulServer.Stop()

	consulInstance := consul.New(consul.ConsulConfig{
		Port: fmt.Sprintf("%d", consulServer.Config.Ports.HTTP),
		Tag:  "marathon",
	})
	app := ConsulApp("serviceA", 2)
	app.Tasks[0].Host = consulServer.Config.Bind
	app.Tasks[1].Host = consulServer.Config.Bind
	marathon := marathon.MarathonerStubWithLeaderForApps("localhost:8080", app)
	sync := New(Config{Leader: "localhost:8080"}, marathon, consulInstance, consulInstance.AddAgentsFromApps)

	// when
	err := sync.SyncServices()

	// then
	assert.NoError(t, err)

	// when
	services, err := consulInstance.GetAllServices()

	// then
	assert.NoError(t, err)
	assert.Len(t, services, 2)

	serviceNames := make(map[string]struct{})
	for _, s := range services {
		serviceNames[s.Name] = struct{}{}
	}
	assert.Len(t, serviceNames, 1)
	assert.Contains(t, serviceNames, "serviceA")
}
