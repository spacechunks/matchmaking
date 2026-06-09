package functional

import (
	"context"
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
	ListeAddr:                            "localhost:60679",
	MatchInterval:                        1 * time.Second,
	AllocateInstanceForPendingMatchAfter: 3 * time.Second,
}

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Suite")
}

var _ = BeforeSuite(func() {
	var (
		logger = slog.New(slog.NewTextHandler(GinkgoWriter, nil))
		serv   = server.New(logger, config, matchmaking.NewStore[matchmaking.Ticket]())
		ctx    = context.Background()
	)

	go func() {
		err := serv.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()
})

var _ = Describe("test matchmaking", func() {
	var client mmv1alpha1.MatchmakingServiceClient

	BeforeEach(func() {
		Eventually(func() error {
			_, err := net.DialTimeout("tcp", "localhost:60679", 10*time.Second)
			return err
		}).WithTimeout(10 * time.Second).Should(Succeed())

		grpcClient, err := grpc.NewClient(config.ListeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).NotTo(HaveOccurred())
		client = mmv1alpha1.NewMatchmakingServiceClient(grpcClient)
	})

	It("matches two tickets of same flavor without delay", func(ctx SpecContext) {
		flavorID := uuid.NewString()

		ticket1 := createAndActivateTicket(ctx, client, flavorID, 1)
		ticket2 := createAndActivateTicket(ctx, client, flavorID, 2)

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
})

func createAndActivateTicket(
	ctx context.Context,
	client mmv1alpha1.MatchmakingServiceClient,
	flavorId string,
	playerCount uint32,
) *mmv1alpha1.Ticket {
	createResp, err := client.CreateTicket(ctx, &mmv1alpha1.CreateTicketRequest{
		FlavorId:    flavorId,
		PlayerCount: playerCount,
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = client.ActivateTicket(ctx, &mmv1alpha1.ActivateTicketRequest{
		TicketId: createResp.Ticket.Id,
	})
	Expect(err).NotTo(HaveOccurred())

	return createResp.Ticket
}

func fetchTicket(ctx context.Context, client mmv1alpha1.MatchmakingServiceClient, ticketID string) *mmv1alpha1.Ticket {
	resp, err := client.GetTicket(ctx, &mmv1alpha1.GetTicketRequest{
		TicketId: ticketID,
	})
	Expect(err).NotTo(HaveOccurred())
	return resp.Ticket
}
