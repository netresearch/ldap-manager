package ldap_cache

import (
	"sync"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

type Cache struct {
	stop chan struct{}

	m      sync.RWMutex
	client *ldap.LDAP
	users  []ldap.User
	groups []ldap.Group
}

type FullLDAPUser struct {
	ldap.User
	Groups []ldap.Group
}

type FullLDAPGroup struct {
	ldap.Group
	Members []ldap.User
}

func New(client *ldap.LDAP) *Cache {
	return &Cache{
		stop:   make(chan struct{}),
		client: client,
		users:  make([]ldap.User, 0),
		groups: make([]ldap.Group, 0),
	}
}

func (l *Cache) Run() {
	t := time.NewTicker(30 * time.Second)

	l.refresh()

	for {
		select {
		case <-l.stop:
			t.Stop()
			log.Info().Msg("LDAP cache stopped")

			return
		case <-t.C:
			l.refresh()
		}
	}
}

func (l *Cache) Stop() {
	l.stop <- struct{}{}
}

func (l *Cache) refreshUsers() error {
	users, err := l.client.FindUsers()
	if err != nil {
		return err
	}

	l.m.Lock()
	l.users = users
	l.m.Unlock()

	return nil
}

func (l *Cache) refreshGroups() error {
	groups, err := l.client.FindGroups()
	if err != nil {
		return err
	}

	l.m.Lock()
	l.groups = groups
	l.m.Unlock()

	return nil
}

func (l *Cache) refresh() {
	if err := l.refreshUsers(); err != nil {
		log.Error().Err(err).Send()
	}

	if err := l.refreshGroups(); err != nil {
		log.Error().Err(err).Send()
	}

	log.Debug().Msgf("Refreshed LDAP cache with %d users and %d groups", len(l.users), len(l.groups))
}

func (l *Cache) FindUsers() []ldap.User {
	l.m.RLock()
	defer l.m.RUnlock()

	return l.users
}

func (l *Cache) FindUserByDN(dn string) (ldap.User, error) {
	l.m.RLock()
	defer l.m.RUnlock()

	for _, user := range l.users {
		if user.DN == dn {
			return user, nil
		}
	}

	return ldap.User{}, ldap.ErrUserNotFound
}

func (l *Cache) FindUserBySAMAccountName(samAccountName string) (ldap.User, error) {
	l.m.RLock()
	defer l.m.RUnlock()

	for _, user := range l.users {
		if user.SAMAccountName == samAccountName {
			return user, nil
		}
	}

	return ldap.User{}, ldap.ErrUserNotFound
}

func (l *Cache) FindGroups() []ldap.Group {
	l.m.RLock()
	defer l.m.RUnlock()

	return l.groups
}

func (l *Cache) FindGroupByDN(dn string) (ldap.Group, error) {
	l.m.RLock()
	defer l.m.RUnlock()

	for _, group := range l.groups {
		if group.DN == dn {
			return group, nil
		}
	}

	return ldap.Group{}, ldap.ErrGroupNotFound
}

func (l *Cache) PopulateGroupsForUser(user *ldap.User) *FullLDAPUser {
	full := &FullLDAPUser{
		User:   *user,
		Groups: make([]ldap.Group, 0),
	}

	for _, groupDN := range user.Groups {
		group, err := l.FindGroupByDN(groupDN)
		if err == nil {
			full.Groups = append(full.Groups, group)
		}
	}

	return full
}

func (l *Cache) PopulateUsersForGroup(group *ldap.Group) *FullLDAPGroup {
	full := &FullLDAPGroup{
		Group:   *group,
		Members: make([]ldap.User, 0),
	}

	for _, userDN := range group.Members {
		user, err := l.FindUserByDN(userDN)
		if err == nil {
			full.Members = append(full.Members, user)
		}
	}

	return full
}
