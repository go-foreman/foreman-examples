package user

import (
	"context"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"sync"
)

type User struct {
	ID    string
	Email string
}

type UserService struct {
	mutex *sync.RWMutex
	users map[string]*User
}

func NewUserService() *UserService {
	return &UserService{users: make(map[string]*User), mutex: &sync.RWMutex{}}
}

func (s *UserService) Register(ctx context.Context, user User) (*User, error) {
	if user.Email == "" {
		return nil, errors.New("email can't be empty")
	}

	if user.ID != "" {
		return nil, errors.New("id must be empty")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	user.ID = uuid.New().String()

	s.users[user.ID] = &user

	return &user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.users[id]

	if !exists {
		return errors.New("user does not exist")
	}

	delete(s.users, id)

	return nil
}

func (s UserService) GetUser(ctx context.Context, id string) (*User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	u, exists := s.users[id]

	if !exists {
		return nil, nil
	}

	return u, nil
}
