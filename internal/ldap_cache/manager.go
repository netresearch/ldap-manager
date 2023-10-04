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
}

type FullLDAPUser struct {
	ldap.User
	Groups []ldap.Group
}

type FullLDAPGroup struct {
	ldap.Group
	Members []ldap.User
}

func New(client *ldap.LDAP) *Manager {
	return &Manager{
		stop:      make(chan struct{}),
		client:    client,
		Users:     NewCached[ldap.User](),
		Groups:    NewCached[ldap.Group](),
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

func (m *Manager) refresh() {
	if err := m.refreshUsers(); err != nil {
		log.Error().Err(err).Send()
	}

	if err := m.refreshGroups(); err != nil {
		log.Error().Err(err).Send()
	}

	log.Debug().Msgf("Refreshed LDAP cache with %d users and %d groups", m.Users.Count(), m.Groups.Count())
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
