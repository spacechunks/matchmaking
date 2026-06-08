package matchmaking

//func TestMatchmaking(t *testing.T) {
//
//	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
//	tickets := &TicketStore{
//		tickets: map[string]Ticket{},
//		mu:      sync.Mutex{},
//	}
//	matches := &MatchStore{
//		match: map[string]Match{},
//		mu:    sync.Mutex{},
//	}
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
//	defer cancel()
//
//	mm := NewFlavorMatchMaker(
//		logger,
//		500*time.Millisecond,
//		tickets,
//		matches,
//	)
//
//	t.Cleanup(func() {
//		mm.Stop()
//	})
//
//	go func() {
//		mm.Start(ctx)
//	}()
//
//	tickets.Add(Ticket{
//		ID:          "1",
//		PlayerCount: 1,
//		FlavorID:    "flavor1",
//	})
//
//	time.Sleep(3 * time.Second)
//
//	tickets.Add(Ticket{
//		ID:          "2",
//		PlayerCount: 3,
//		FlavorID:    "flavor1",
//	})
//
//	time.Sleep(3 * time.Second)
//
//	tickets.Add(Ticket{
//		ID:          "3",
//		PlayerCount: 3,
//		FlavorID:    "flavor1",
//	})
//
//	time.Sleep(4 * time.Second)
//
//	tickets.InvalidateTicket("3")
//
//	time.Sleep(3 * time.Minute)
//
//	cancel()
//}
