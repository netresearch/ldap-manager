package ldap_cache

import (
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

type Manager struct {
	stop chan struct{}

	client *ldap.LDAP

	Users     Cache[ldap.User]
	Groups    Cache[ldap.Group]
	Computers Cache[ldap.Computer]
}

type FullLDAPUser struct {
	ldap.User
	Groups []ldap.Group
}

type FullLDAPGroup struct {
	ldap.Group
	Members []ldap.User
}

type FullLDAPComputer struct {
	ldap.Computer
	Groups []ldap.Group
}

func New(client *ldap.LDAP) *Manager {
	return &Manager{
		stop:      make(chan struct{}),
		client:    client,
		Users:     NewCached[ldap.User](),
		Groups:    NewCached[ldap.Group](),
		Computers: NewCached[ldap.Computer](),
	}
}

func (m *Manager) Run() {
	t := time.NewTicker(30 * time.Second)

	m.refresh()

	for {
		select {
		case <-m.stop:
			t.Stop()
			log.Info().Msg("LDAP cache stopped")

			return
		case <-t.C:
			m.refresh()
		}
	}
}

func (m *Manager) Stop() {
	m.stop <- struct{}{}
}

func (m *Manager) refreshUsers() error {
	users, err := m.client.FindUsers()
	if err != nil {
		return err
	}

	m.Users.set(users)

	return nil
}

func (m *Manager) refreshGroups() error {
	groups, err := m.client.FindGroups()
	if err != nil {
		return err
	}

	m.Groups.set(groups)

	return nil
}

func (m *Manager) refreshComputers() error {
	computers, err := m.client.FindComputers()
	if err != nil {
		return err
	}

	m.Computers.set(computers)

	return nil
}

func (m *Manager) refresh() {
	if err := m.refreshUsers(); err != nil {
		log.Error().Err(err).Send()
	}

	if err := m.refreshGroups(); err != nil {
		log.Error().Err(err).Send()
	}

	if err := m.refreshComputers(); err != nil {
		log.Error().Err(err).Send()
	}

	log.Debug().Msgf("Refreshed LDAP cache with %d users, %d groups and %d computers", m.Users.Count(), m.Groups.Count(), m.Computers.Count())
}

func (m *Manager) FindUsers() []ldap.User {
	return m.Users.Get()
}

func (m *Manager) FindUserByDN(dn string) (*ldap.User, error) {
	user, found := m.Users.FindByDN(dn)
	if !found {
		return nil, ldap.ErrUserNotFound
	}

	return user, nil
}

func (m *Manager) FindUserBySAMAccountName(samAccountName string) (*ldap.User, error) {
	user, found := m.Users.Find(func(user ldap.User) bool {
		return user.SAMAccountName == samAccountName
	})
	if !found {
		return nil, ldap.ErrUserNotFound
	}

	return user, nil
}

func (m *Manager) FindGroups() []ldap.Group {
	return m.Groups.Get()
}

func (m *Manager) FindGroupByDN(dn string) (*ldap.Group, error) {
	group, found := m.Groups.FindByDN(dn)

	if !found {
		return nil, ldap.ErrGroupNotFound
	}

	return group, nil
}

func (m *Manager) FindComputers() []ldap.Computer {
	return m.Computers.Get()
}

func (m *Manager) FindComputerByDN(dn string) (*ldap.Computer, error) {
	computer, found := m.Computers.FindByDN(dn)
	if !found {
		return nil, ldap.ErrComputerNotFound
	}

	return computer, nil
}

func (m *Manager) PopulateGroupsForUser(user *ldap.User) *FullLDAPUser {
	full := &FullLDAPUser{
		User:   *user,
		Groups: make([]ldap.Group, 0),
	}

	for _, groupDN := range user.Groups {
		group, err := m.FindGroupByDN(groupDN)
		if err == nil {
			full.Groups = append(full.Groups, *group)
		}
	}

	return full
}

func (m *Manager) PopulateUsersForGroup(group *ldap.Group) *FullLDAPGroup {
	full := &FullLDAPGroup{
		Group:   *group,
		Members: make([]ldap.User, 0),
	}

	for _, userDN := range group.Members {
		user, err := m.FindUserByDN(userDN)
		if err == nil {
			full.Members = append(full.Members, *user)
		}
	}

	return full
}

func (m *Manager) PopulateGroupsForComputer(computer *ldap.Computer) *FullLDAPComputer {
	full := &FullLDAPComputer{
		Computer: *computer,
		Groups:   make([]ldap.Group, 0),
	}

	for _, groupDN := range computer.Groups {
		group, err := m.FindGroupByDN(groupDN)
		if err == nil {
			full.Groups = append(full.Groups, *group)
		}
	}

	return full
}
