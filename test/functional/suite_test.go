package functional

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mmv1alpha1 "github.com/spacechunks/matchmaking/api/v1alpha1"
	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"github.com/spacechunks/matchmaking/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var config = server.Config{
	MatchInterval:                        1 * time.Second,
	AllocateInstanceForPendingMatchAfter: 3 * time.Second,
}

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Suite")
}

var _ = Describe("matchmaking", func() {
	var client mmv1alpha1.MatchmakingServiceClient
	serverCtx, serverCancel := context.WithCancel(context.Background())

	BeforeEach(func(ctx context.Context) {
		port := 60000 + GinkgoParallelProcess()
		addr := fmt.Sprintf("localhost:%d", port)

		config.ListeAddr = addr

		var (
			logger = slog.New(slog.NewTextHandler(GinkgoWriter, nil))
			serv   = server.New(logger, config, matchmaking.NewStore[matchmaking.Ticket]())
		)

		go func() {
			err := serv.Run(serverCtx)
			Expect(err).NotTo(HaveOccurred())
		}()

		Eventually(func() error {
			_, err := net.DialTimeout("tcp", addr, 10*time.Second)
			return err
		}).WithTimeout(10 * time.Second).Should(Succeed())

		grpcClient, err := grpc.NewClient(config.ListeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).NotTo(HaveOccurred())

		client = mmv1alpha1.NewMatchmakingServiceClient(grpcClient)
	})

	AfterEach(func(ctx context.Context) {
		serverCancel()
	})

	It("matches two tickets of same flavor without delay", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 2)
		)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
			)

			g.Expect(updated1.Assignment).NotTo(BeNil())
			g.Expect(updated2.Assignment).NotTo(BeNil())
			g.Expect(updated1.Assignment.InstanceId).To(Equal(updated2.Assignment.InstanceId))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("matches multiple tickets of same flavor with delay", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
		)

		time.Sleep(1 * time.Second)

		ticket2 := createAndActivateTicket(ctx, client, flavorID, 1)

		time.Sleep(1 * time.Second)

		ticket3 := createAndActivateTicket(ctx, client, flavorID, 1)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
				updated3 = fetchTicket(ctx, client, ticket3.Id)
			)

			g.Expect(updated1.Assignment).NotTo(BeNil())
			g.Expect(updated2.Assignment).NotTo(BeNil())
			g.Expect(updated3.Assignment).NotTo(BeNil())

			insID := updated1.Assignment.InstanceId
			g.Expect(updated2.Assignment.InstanceId).To(Equal(insID))
			g.Expect(updated3.Assignment.InstanceId).To(Equal(insID))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("matches multiple tickets of different flavors", func(ctx SpecContext) {
		var (
			flavorID1 = uuid.NewString()
			ticket1   = createAndActivateTicket(ctx, client, flavorID1, 1)
			ticket2   = createAndActivateTicket(ctx, client, flavorID1, 2)

			flavorID2 = uuid.NewString()
			ticket3   = createAndActivateTicket(ctx, client, flavorID2, 1)
			ticket4   = createAndActivateTicket(ctx, client, flavorID2, 2)
		)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
				updated3 = fetchTicket(ctx, client, ticket3.Id)
				updated4 = fetchTicket(ctx, client, ticket4.Id)
			)

			g.Expect(updated1.Assignment).NotTo(BeNil())
			g.Expect(updated2.Assignment).NotTo(BeNil())
			g.Expect(updated3.Assignment).NotTo(BeNil())
			g.Expect(updated4.Assignment).NotTo(BeNil())

			g.Expect(updated1.Assignment.InstanceId).To(Equal(updated2.Assignment.InstanceId))
			g.Expect(updated3.Assignment.InstanceId).To(Equal(updated4.Assignment.InstanceId))

			// make sure they do not equal one another, these should be two distinct matches
			g.Expect(updated1.Assignment.InstanceId).NotTo(Equal(updated3.Assignment.InstanceId))
			g.Expect(updated4.Assignment.InstanceId).NotTo(Equal(updated2.Assignment.InstanceId))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("does not match tickets with different flavor ids", func(ctx SpecContext) {
		var (
			flavorID1 = uuid.NewString()
			flavorID2 = uuid.NewString()
			ticket1   = createAndActivateTicket(ctx, client, flavorID1, 1)
			ticket2   = createAndActivateTicket(ctx, client, flavorID2, 2)
		)

		Consistently(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
			)

			g.Expect(updated1.Assignment).To(BeNil())
			g.Expect(updated2.Assignment).To(BeNil())
		}, "5s").Should(Succeed())
	})

	It("should not match deactivated tickets", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
			ticket2  = createTicket(ctx, client, flavorID, 20)
			ticket3  = createTicket(ctx, client, flavorID, 10)
		)

		Consistently(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
				updated3 = fetchTicket(ctx, client, ticket3.Id)
			)

			g.Expect(updated1.Assignment).To(BeNil())
			g.Expect(updated2.Assignment).To(BeNil())
			g.Expect(updated3.Assignment).To(BeNil())
		}, "5s").Should(Succeed())
	})

	It("matches a ticket that has been activated", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
			ticket2  = createTicket(ctx, client, flavorID, 2)
		)

		time.Sleep(3 * time.Second)

		activateTicket(ctx, client, ticket2.Id)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
			)

			g.Expect(updated1.Assignment).NotTo(BeNil())
			g.Expect(updated2.Assignment).NotTo(BeNil())
			g.Expect(updated1.Assignment.InstanceId).To(Equal(updated2.Assignment.InstanceId))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("creates 2 different matches when combined player count is greater than maxPlayer", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 9)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 3)
			ticket3  = createAndActivateTicket(ctx, client, flavorID, 3)
		)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
				updated3 = fetchTicket(ctx, client, ticket3.Id)
			)

			g.Expect(updated1.Assignment).NotTo(BeNil())
			g.Expect(updated2.Assignment).NotTo(BeNil())
			g.Expect(updated3.Assignment).NotTo(BeNil())

			// ticket1 is a separate match
			g.Expect(updated1.Assignment.InstanceId).NotTo(Equal(updated2.Assignment.InstanceId))

			// ticket2 and ticket3 should be matched together
			g.Expect(updated2.Assignment.InstanceId).To(Equal(updated3.Assignment.InstanceId))

		}, "10s").Should(Succeed())
	})

	It("creates a match for a ticket that has maxPlayers while others wait", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 10)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 1)
			ticket3  = createAndActivateTicket(ctx, client, flavorID, 1)
		)

		Eventually(func(g Gomega) {
			updated := fetchTicket(ctx, client, ticket1.Id)
			g.Expect(updated.Assignment).NotTo(BeNil())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		Consistently(func(g Gomega) {
			var (
				updated2 = fetchTicket(ctx, client, ticket2.Id)
				updated3 = fetchTicket(ctx, client, ticket3.Id)
			)
			g.Expect(updated2.Assignment).To(BeNil())
			g.Expect(updated3.Assignment).To(BeNil())
		}, "5s").Should(Succeed())
	})

	It("should not match tickets if combined playerCounts cannot reach minPlayers", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 1)
		)

		Consistently(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
			)

			g.Expect(updated1.Assignment).To(BeNil())
			g.Expect(updated2.Assignment).To(BeNil())
		}, "5s").Should(Succeed())
	})

	// TODO: test that after min players change old run with the old flavor version and new ones with the new
})

func createAndActivateTicket(
	ctx context.Context,
	client mmv1alpha1.MatchmakingServiceClient,
	flavorId string,
	playerCount uint32,
) *mmv1alpha1.Ticket {
	t := createTicket(ctx, client, flavorId, playerCount)
	activateTicket(ctx, client, t.Id)
	return t
}

func createTicket(ctx context.Context,
	client mmv1alpha1.MatchmakingServiceClient,
	flavorId string,
	playerCount uint32,
) *mmv1alpha1.Ticket {
	createResp, err := client.CreateTicket(ctx, &mmv1alpha1.CreateTicketRequest{
		FlavorId:    flavorId,
		PlayerCount: playerCount,
	})
	Expect(err).NotTo(HaveOccurred())

	return createResp.Ticket
}

func activateTicket(ctx context.Context, client mmv1alpha1.MatchmakingServiceClient, ticketID string) {
	_, err := client.ActivateTicket(ctx, &mmv1alpha1.ActivateTicketRequest{
		TicketId: ticketID,
	})
	Expect(err).NotTo(HaveOccurred())
}

func fetchTicket(ctx context.Context, client mmv1alpha1.MatchmakingServiceClient, ticketID string) *mmv1alpha1.Ticket {
	resp, err := client.GetTicket(ctx, &mmv1alpha1.GetTicketRequest{
		TicketId: ticketID,
	})
	Expect(err).NotTo(HaveOccurred())
	return resp.Ticket
}
