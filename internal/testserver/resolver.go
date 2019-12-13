package testserver

import (
	"context"
	"time"
) // THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

type Resolver struct {
	TodoStore []*Todo
}

func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}
func (r *Resolver) Subscription() SubscriptionResolver {
	return &subscriptionResolver{r}
}

type mutationResolver struct{ *Resolver }

func (r *mutationResolver) CreateTodo(ctx context.Context, input NewTodo) (*Todo, error) {
	panic("not implemented")
}

type queryResolver struct{ *Resolver }

func (r *queryResolver) Todos(ctx context.Context) ([]*Todo, error) {
	return []*Todo{}, nil
}

type subscriptionResolver struct{ *Resolver }

func (r *subscriptionResolver) Todos(ctx context.Context) (<-chan []*Todo, error) {
	ch := make(chan []*Todo, 1)

	go func() {
		for _, todo := range r.TodoStore {
			ch <- []*Todo{todo}
			time.Sleep(time.Millisecond)
		}
	}()

	return ch, nil
}
