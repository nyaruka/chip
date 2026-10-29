package email

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
)

const (
	sweepInterval  = 5 * time.Second // sets how late an email to a visitor who isn't connected can be
	sweepBatchSize = 100
)

// how long to wait before each retry of a batch whose send failed in a way that may not fail again - once these run
// out, the batch is dropped
var retryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// Sweeper emails conversations their held back messages once they're due. Every instance runs one: claiming is
// atomic so a conversation is only ever emailed by one of them, and a crash mid-send loses that email rather than
// sending it twice.
type Sweeper struct {
	rt *runtime.Runtime

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSweeper creates a new sweeper
func NewSweeper(rt *runtime.Runtime) *Sweeper {
	ctx, cancel := context.WithCancel(context.Background())
	return &Sweeper{rt: rt, ctx: ctx, cancel: cancel}
}

// Start starts sweeping in the background
func (s *Sweeper) Start() {
	s.wg.Go(func() {
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.sweep()
			}
		}
	})
}

func (s *Sweeper) sweep() {
	defer func() {
		if val := recover(); val != nil {
			runtime.PanicHandler(val, map[string]string{"comp": "email"})
		}
	}()

	if err := s.Tick(s.ctx); err != nil {
		slog.Error("error sweeping emails", "comp", "email", "error", err)
	}
}

// Stop stops sweeping, waiting for a sweep in progress to finish the batch it's on
func (s *Sweeper) Stop() {
	s.cancel()
	s.wg.Wait()
}

// Tick emails every conversation that's due
func (s *Sweeper) Tick(ctx context.Context) error {
	vc, err := s.rt.VK.GetContext(ctx)
	if err != nil {
		return err
	}
	defer vc.Close()

	for ctx.Err() == nil {
		members, err := dueMembers(vc, sweepBatchSize)
		if err != nil {
			return fmt.Errorf("error fetching due emails: %w", err)
		}

		for _, member := range members {
			if ctx.Err() != nil {
				break
			}

			b, err := claim(vc, member)
			if err != nil {
				return fmt.Errorf("error claiming due email: %w", err)
			}
			if b != nil {
				s.process(context.WithoutCancel(ctx), b)
			}
		}

		// claiming removes a conversation from those due, whoever wins it, so a full page means there may be more
		if len(members) < sweepBatchSize {
			break
		}
	}
	return nil
}

// process sends a claimed batch, requeueing it to try again if that failed in a way which may not fail again
func (s *Sweeper) process(ctx context.Context, b *batch) {
	log := slog.With("comp", "email", "channel", b.ChannelUUID, "chat_id", b.ChatID, "attempts", b.Attempts)

	err := s.send(ctx, b)
	if err == nil {
		return
	}

	if !IsRetryable(err) || b.Attempts >= len(retryDelays) {
		log.Error("error sending email, giving up", "error", err)
		return
	}

	delay := retryDelays[b.Attempts]
	b.Attempts++

	vc, vcErr := s.rt.VK.GetContext(ctx)
	if vcErr == nil {
		defer vc.Close()
		vcErr = requeue(vc, b, delay)
	}
	if vcErr != nil {
		log.Error("error requeueing email, giving up", "error", err, "requeue_error", vcErr)
	} else {
		log.Warn("error sending email, will retry", "error", err, "delay", delay)
	}
}

// send emails the given batch's messages that the visitor still hasn't read. A conversation that can no longer be
// emailed - its channel or contact gone, or its channel no longer configured for email - is quietly skipped.
func (s *Sweeper) send(ctx context.Context, b *batch) error {
	ch, err := models.LoadChannel(ctx, s.rt.DB, b.ChannelUUID)
	if err != nil {
		return err
	}
	if ch == nil {
		return nil
	}
	cfg := ch.EmailConfig()
	if cfg == nil {
		return nil
	}

	visitor, err := models.LoadVisitor(ctx, s.rt.DB, ch, b.ChatID)
	if err != nil || visitor == nil {
		return err
	}

	msgs, err := models.GetUnreadChatMsgs(ctx, s.rt.DB, ch, visitor, b.MsgUUIDs)
	if err != nil || len(msgs) == 0 {
		return err
	}

	org, err := models.LoadOrg(ctx, s.rt.DB, ch.OrgID())
	if err != nil || org == nil {
		return err
	}

	e, err := Render(b.Address, org.Name(), org.Timezone(), msgs)
	if err != nil {
		return fmt.Errorf("%w: error rendering: %w", ErrUnsendable, err)
	}

	return Send(ctx, s.rt, ch, cfg, org.Name(), e)
}
