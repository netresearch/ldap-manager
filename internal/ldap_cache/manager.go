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

	m.Refresh()

	for {
		select {
		case <-m.stop:
			t.Stop()
			log.Info().Msg("LDAP cache stopped")

			return
		case <-t.C:
			m.Refresh()
		}
	}
}

func (m *Manager) Stop() {
	m.stop <- struct{}{}
}

func (m *Manager) RefreshUsers() error {
	users, err := m.client.FindUsers()
	if err != nil {
		return err
	}

	m.Users.setAll(users)

	return nil
}

func (m *Manager) RefreshGroups() error {
	groups, err := m.client.FindGroups()
	if err != nil {
		return err
	}

	m.Groups.setAll(groups)

	return nil
}

func (m *Manager) RefreshComputers() error {
	computers, err := m.client.FindComputers()
	if err != nil {
		return err
	}

	m.Computers.setAll(computers)

	return nil
}

func (m *Manager) Refresh() {
	if err := m.RefreshUsers(); err != nil {
		log.Error().Err(err).Send()
	}

	if err := m.RefreshGroups(); err != nil {
		log.Error().Err(err).Send()
	}

	if err := m.RefreshComputers(); err != nil {
		log.Error().Err(err).Send()
	}

	log.Debug().Msgf("Refreshed LDAP cache with %d users, %d groups and %d computers", m.Users.Count(), m.Groups.Count(), m.Computers.Count())
}

func (m *Manager) FindUsers(showDisabled bool) []ldap.User {
	if !showDisabled {
		return m.Users.Filter(func(t ldap.User) bool {
			return t.Enabled
		})
	}

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

func (m *Manager) FindComputers(showDisabled bool) []ldap.Computer {
	if !showDisabled {
		return m.Computers.Filter(func(t ldap.Computer) bool {
			return t.Enabled
		})
	}

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

func (m *Manager) PopulateUsersForGroup(group *ldap.Group, showDisabled bool) *FullLDAPGroup {
	full := &FullLDAPGroup{
		Group:   *group,
		Members: make([]ldap.User, 0),
	}

	for _, userDN := range group.Members {
		user, err := m.FindUserByDN(userDN)
		if err == nil {
			if showDisabled && !user.Enabled {
				continue
			}

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

func (m *Manager) OnAddUserToGroup(userDN string, groupDN string) {
	m.Users.update(func(user *ldap.User) {
		if user.DN() != userDN {
			return
		}

		user.Groups = append(user.Groups, groupDN)
	})

	m.Groups.update(func(group *ldap.Group) {
		if group.DN() != groupDN {
			return
		}

		group.Members = append(group.Members, userDN)
	})
}

func (m *Manager) OnRemoveUserFromGroup(userDN string, groupDN string) {
	m.Users.update(func(user *ldap.User) {
		if user.DN() != userDN {
			return
		}

		for idx, group := range user.Groups {
			if group == groupDN {
				user.Groups = append(user.Groups[:idx], user.Groups[idx+1:]...)
			}
		}
	})

	m.Groups.update(func(group *ldap.Group) {
		if group.DN() != groupDN {
			return
		}

		for idx, member := range group.Members {
			if member == userDN {
				group.Members = append(group.Members[:idx], group.Members[idx+1:]...)
			}
		}
	})
}
