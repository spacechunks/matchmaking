package functional

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	mmv1alpha1 "github.com/spacechunks/matchmaking/api/v1alpha1"
	"github.com/spacechunks/matchmaking/internal/matchmaking"
	"github.com/spacechunks/matchmaking/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	config = server.Config{
		MatchInterval:                        100 * time.Millisecond,
		AllocateInstanceForPendingMatchAfter: 1 * time.Second,
		RemoveInactiveTicketsAfter:           3 * time.Second,
	}

	workingFlavor1 = &chunkv1alpha1.Flavor{
		Id:   uuid.NewString(),
		Name: "flavor1",
		Versions: []*chunkv1alpha1.FlavorVersion{
			{
				Id:          uuid.NewString(),
				Version:     "non-playable-version",
				BuildStatus: chunkv1alpha1.BuildStatus_CHECKPOINT_BUILD,
				MinPlayers:  10,
				MaxPlayers:  10,
			},
			// this one should always be picked
			{
				Id:          uuid.NewString(),
				Version:     "playable-version",
				BuildStatus: chunkv1alpha1.BuildStatus_COMPLETED,
				MinPlayers:  3,
				MaxPlayers:  10,
			},
		},
	}

	workingFlavor2 = &chunkv1alpha1.Flavor{
		Id:   uuid.NewString(),
		Name: "flavor2",
		Versions: []*chunkv1alpha1.FlavorVersion{
			{
				Id:          uuid.NewString(),
				Version:     "playable-version",
				BuildStatus: chunkv1alpha1.BuildStatus_COMPLETED,
				MinPlayers:  3,
				MaxPlayers:  10,
			},
		},
	}

	nonPlayableVersionsFlavor = &chunkv1alpha1.Flavor{
		Id:   uuid.NewString(),
		Name: "flavor3-non-playable",
		Versions: []*chunkv1alpha1.FlavorVersion{
			{
				Id:          uuid.NewString(),
				Version:     "non-playable-version1",
				BuildStatus: chunkv1alpha1.BuildStatus_CHECKPOINT_BUILD_FAILED,
				MinPlayers:  3,
				MaxPlayers:  10,
			},
			{
				Id:          uuid.NewString(),
				Version:     "non-playable-version2",
				BuildStatus: chunkv1alpha1.BuildStatus_IMAGE_BUILD_FAILED,
				MinPlayers:  3,
				MaxPlayers:  10,
			},
		},
	}
)

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional Suite")
}

var _ = Describe("matchmaking", func() {
	var client mmv1alpha1.MatchmakingServiceClient
	serverCtx := context.Background()

	BeforeEach(func(ctx context.Context) {
		config.ListeAddr = fmt.Sprintf("localhost:%d", 30000+rand.IntN(10000))
		config.ControlPlaneAddr = fmt.Sprintf("localhost:%d", 31000+rand.IntN(10000))

		var (
			logger  = slog.New(slog.NewTextHandler(GinkgoWriter, nil))
			serv    = server.New(logger, config, matchmaking.NewStore[matchmaking.Ticket]())
			fakceCP = FakeControlPlane{
				flavors: map[string]*chunkv1alpha1.Flavor{
					workingFlavor1.Id:            workingFlavor1,
					workingFlavor2.Id:            workingFlavor2,
					nonPlayableVersionsFlavor.Id: nonPlayableVersionsFlavor,
				},
				listenAddr: config.ControlPlaneAddr,
			}
		)

		go func() {
			defer GinkgoRecover()
			err := fakceCP.Run(serverCtx)
			Expect(err).NotTo(HaveOccurred())
		}()

		Eventually(func() error {
			_, err := net.DialTimeout("tcp", config.ControlPlaneAddr, 10*time.Second)
			GinkgoWriter.Println(err)
			return err
		}).WithTimeout(10 * time.Second).Should(Succeed())

		go func() {
			defer GinkgoRecover()
			err := serv.Run(serverCtx)
			Expect(err).NotTo(HaveOccurred())
		}()

		Eventually(func() error {
			_, err := net.DialTimeout("tcp", config.ListeAddr, 10*time.Second)
			return err
		}).WithTimeout(10 * time.Second).Should(Succeed())

		grpcClient, err := grpc.NewClient(config.ListeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).NotTo(HaveOccurred())

		client = mmv1alpha1.NewMatchmakingServiceClient(grpcClient)
	})

	AfterEach(func(ctx context.Context) {
		//serverCancel()
	})

	It("matches two tickets of same flavor without delay", func(ctx SpecContext) {
		var (
			flavorID = workingFlavor1.Id
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
			flavorID = workingFlavor1.Id
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
		)

		time.Sleep(100 * time.Millisecond)

		ticket2 := createAndActivateTicket(ctx, client, flavorID, 1)

		time.Sleep(100 * time.Millisecond)

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
			flavorID1 = workingFlavor1.Id
			ticket1   = createAndActivateTicket(ctx, client, flavorID1, 1)
			ticket2   = createAndActivateTicket(ctx, client, flavorID1, 2)

			flavorID2 = workingFlavor2.Id
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
			flavorID1 = workingFlavor1.Id
			flavorID2 = workingFlavor2.Id
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
			flavorID = workingFlavor1.Id
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
		}, "2s").Should(Succeed())
	})

	It("matches a ticket that has been activated", func(ctx SpecContext) {
		var (
			flavorID = workingFlavor1.Id
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 1)
			ticket2  = createTicket(ctx, client, flavorID, 2)
		)

		time.Sleep(300 * time.Millisecond)

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
			flavorID = workingFlavor1.Id
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
			flavorID = workingFlavor1.Id
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
		}, "2s").Should(Succeed())
	})

	It("should not match tickets if combined playerCounts cannot reach minPlayers", func(ctx SpecContext) {
		var (
			flavorID = workingFlavor1.Id
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
		}, "2s").Should(Succeed())
	})

	It("removes inactive tickets", func(ctx SpecContext) {
		var (
			flavorID = workingFlavor1.Id
			ticket1  = createTicket(ctx, client, flavorID, 20)
		)

		Eventually(func(g Gomega) {
			_, err := client.GetTicket(ctx, &mmv1alpha1.GetTicketRequest{
				TicketId: ticket1.Id,
			})
			g.Expect(err).To(BeEquivalentTo(status.Error(codes.NotFound, "ticket not found")))
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})

	It("it removes tickets where flavor does not have any playable flavor version", func(ctx SpecContext) {
		var (
			flavorID = nonPlayableVersionsFlavor.Id
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 3)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 3)
		)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
			)

			g.Expect(updated1.Status).To(Equal(mmv1alpha1.TicketStatus_NO_PLAYABLE_FLAVOR_VERSION))
			g.Expect(updated2.Status).To(Equal(mmv1alpha1.TicketStatus_NO_PLAYABLE_FLAVOR_VERSION))
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})

	It("removes tickets where flavor could not be found", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 3)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 3)
		)

		Eventually(func(g Gomega) {
			var (
				updated1 = fetchTicket(ctx, client, ticket1.Id)
				updated2 = fetchTicket(ctx, client, ticket2.Id)
			)

			g.Expect(updated1.Status).To(Equal(mmv1alpha1.TicketStatus_NO_PLAYABLE_FLAVOR_VERSION))
			g.Expect(updated2.Status).To(Equal(mmv1alpha1.TicketStatus_NO_PLAYABLE_FLAVOR_VERSION))
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})

	It("checks that tickets in NO_PLAYABLE_FLAVOR_VERSION cannot be activated again", func(ctx SpecContext) {
		var (
			flavorID = uuid.NewString()
			ticket   = createAndActivateTicket(ctx, client, flavorID, 3)
		)

		Eventually(func(g Gomega) {
			updated := fetchTicket(ctx, client, ticket.Id)
			g.Expect(updated.Status).To(Equal(mmv1alpha1.TicketStatus_NO_PLAYABLE_FLAVOR_VERSION))
		}).WithTimeout(5 * time.Second).Should(Succeed())

		_, err := client.ActivateTicket(ctx, &mmv1alpha1.ActivateTicketRequest{
			TicketId: ticket.Id,
		})
		Expect(err).To(Equal(status.Error(codes.FailedPrecondition, "ticket is already active or has been invalidated")))
	})

	It("deletes the match when a ticket of a match is removed", func(ctx SpecContext) {
		var (
			flavorID = workingFlavor1.Id
			ticket1  = createAndActivateTicket(ctx, client, flavorID, 3)
			ticket2  = createAndActivateTicket(ctx, client, flavorID, 1)
		)

		// wait so we have a match
		time.Sleep(200 * time.Millisecond)

		// this should invalidate the match
		removeTicket(ctx, client, ticket1.Id)

		// give the server some time to remove the ticket from the store
		time.Sleep(50 * time.Millisecond)

		Consistently(func(g Gomega) {
			_, err := client.GetTicket(ctx, &mmv1alpha1.GetTicketRequest{
				TicketId: ticket1.Id,
			})
			g.Expect(err).To(BeEquivalentTo(status.Error(codes.NotFound, "ticket not found")))

			updated2 := fetchTicket(ctx, client, ticket2.Id)
			g.Expect(updated2.Assignment).To(BeNil())
		}, "2s").Should(Succeed())
	})
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

func removeTicket(ctx context.Context, client mmv1alpha1.MatchmakingServiceClient, ticketID string) {
	_, err := client.RemoveTicket(ctx, &mmv1alpha1.RemoveTicketRequest{
		TicketId: ticketID,
	})
	Expect(err).NotTo(HaveOccurred())
}
