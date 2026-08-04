package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	operationComplete   = "sales.complete.v1"
	operationMobileSync = "mobile.sales.sync.v1"
	operationReverse    = "sales.reverse.v1"
)

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, currentTime clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || currentTime == nil {
		return nil, errors.New("sales service dependencies are required")
	}
	return &Service{repository: repository, ids: ids, clock: currentTime}, nil
}

func (s *Service) Complete(ctx context.Context, command CompleteCommand) (Sale, error) {
	command = normalizeCompleteCommand(command)
	if err := command.Validate(); err != nil {
		return Sale{}, err
	}
	if command.Kind == KindCash && !IsCanonicalPaymentMethod(command.PaymentMethod) {
		return Sale{}, ErrUnsupportedPayment
	}
	if command.Offline && command.PaymentMethod != PaymentCash {
		return Sale{}, ErrOfflinePaymentMethod
	}
	requestHash, err := hashCommand(command)
	if err != nil {
		return Sale{}, err
	}
	now := s.clock.Now().UTC()
	documentAt := now
	if command.Offline {
		documentAt = command.ClientTimestamp.UTC()
	}
	var result Sale
	err = s.repository.WithTransaction(ctx, func(tx Transaction) error {
		authorized, err := tx.Authorize(ctx, command.Scope, command.ActorID, "sales.complete")
		if err != nil {
			return err
		}
		if !authorized {
			return ErrForbidden
		}
		operation, idempotencyKey := operationComplete, command.IdempotencyKey
		var enrolled devices.Device
		if command.DeviceID != "" {
			canSync, permissionErr := tx.Authorize(ctx, command.Scope, command.ActorID, "mobile.sales.sync")
			if permissionErr != nil {
				return permissionErr
			}
			if !canSync {
				return ErrForbidden
			}
			enrolled, err = tx.MobileDevice(ctx, command.Scope, command.ActorID, command.DeviceID)
			if err != nil {
				return err
			}
			if enrolled.Status != devices.StatusActive {
				return devices.ErrNotActive
			}
			operation = operationMobileSync
			idempotencyKey = command.DeviceID + "::" + command.ClientTransactionID
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, command.Scope, operation, idempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.Sale(ctx, command.Scope, resultID)
			result.IdempotentReplay = err == nil
			return err
		}
		if command.Offline {
			policy, err := tx.OfflinePostingPolicy(ctx, command.Scope, now)
			if err != nil {
				return err
			}
			if policy.AccountingTimeBasis != AccountingTimeServerReceipt ||
				policy.MaximumFutureSkewSeconds < 0 || policy.MaximumFutureSkewSeconds > 86400 ||
				!policy.RequireSameFiscalPeriod {
				return ErrPostingConfig
			}
			if documentAt.After(now.Add(time.Duration(policy.MaximumFutureSkewSeconds) * time.Second)) {
				return ErrOfflineClockReconciliation
			}
			documentPeriod, err := tx.FiscalPeriod(ctx, command.Scope, documentAt)
			if err != nil {
				return err
			}
			accountingPeriod, err := tx.FiscalPeriod(ctx, command.Scope, now)
			if err != nil {
				return err
			}
			if !accountingPeriod.Open || documentPeriod.ID == "" || documentPeriod.ID != accountingPeriod.ID {
				return ErrOfflinePeriodReconciliation
			}
		}
		if command.DeviceID != "" {
			if command.Offline {
				if !enrolled.OfflineEnabled {
					return devices.ErrOfflineDisabled
				}
				valid, err := tx.OfflineLeaseValid(ctx, devices.OfflineLease{
					Scope: command.Scope, DeviceID: command.DeviceID, AppVersion: strings.TrimSpace(command.AppVersion),
					MasterDataVersion: command.MasterDataVersion, PriceVersion: command.PriceVersion,
					CatalogSnapshotToken: command.CatalogSnapshotToken,
				}, command.ClientTimestamp.UTC())
				if err != nil {
					return err
				}
				if !valid {
					return devices.ErrOfflineLeaseExpired
				}
			} else if strings.TrimSpace(command.AppVersion) != enrolled.AppVersion ||
				command.MasterDataVersion != enrolled.MasterDataVersion || command.PriceVersion != enrolled.PriceVersion ||
				command.CatalogSnapshotToken != enrolled.CatalogSnapshotToken ||
				enrolled.MasterDataVersion != enrolled.AvailableMasterDataVersion ||
				enrolled.PriceVersion != enrolled.AvailablePriceVersion ||
				enrolled.CatalogSnapshotToken != enrolled.AvailableCatalogSnapshotToken {
				return devices.ErrStaleMasterData
			}
		}

		var customer customers.Account
		if command.Offline {
			customer, err = tx.OfflineCatalogCustomer(ctx, command.Scope, command.CatalogSnapshotToken, command.CustomerID)
		} else {
			customer, err = tx.Customer(ctx, command.Scope, command.CustomerID)
		}
		if err != nil {
			return err
		}
		if !customer.Active {
			return ErrCustomerInactive
		}
		if command.Kind == KindCredit {
			if customer.General {
				return ErrGeneralCustomerCredit
			}
			if !customer.CreditEnabled {
				return ErrCustomerCreditDisabled
			}
		}
		open, err := tx.FiscalPeriodOpen(ctx, command.Scope, now)
		if err != nil {
			return err
		}
		if !open {
			return ErrFiscalPeriodClosed
		}

		posting, err := tx.SalesPostingConfig(ctx, command.Scope)
		if err != nil {
			return err
		}
		if posting.ReceivableAccountID == "" || posting.TaxPayableAccountID == "" {
			return ErrPostingConfig
		}
		cashAccount := ""
		if command.Kind == KindCash {
			cashAccount = posting.CashAccounts[command.PaymentMethod]
			if cashAccount == "" {
				return ErrUnsupportedPayment
			}
		}

		saleID, err := s.ids.New()
		if err != nil {
			return err
		}
		sale := Sale{
			ID: saleID, Scope: command.Scope, RecordType: RecordSale,
			Kind: command.Kind, Status: StatusPosted, CustomerID: customer.ID,
			PaymentMethod: command.PaymentMethod, DeviceID: command.DeviceID,
			ClientTransactionID: command.ClientTransactionID, Offline: command.Offline,
			CreatedBy: command.ActorID, DocumentAt: documentAt, ReceivedAt: now,
			AccountingAt: now, AccountingTimeBasis: AccountingTimeServerReceipt, CreatedAt: now,
		}
		if command.DeviceID != "" {
			clientTimestamp := command.ClientTimestamp.UTC()
			sale.ClientTimestamp = &clientTimestamp
			sale.AppVersion = strings.TrimSpace(command.AppVersion)
			sale.MasterDataVersion = command.MasterDataVersion
			sale.PriceVersion = command.PriceVersion
			sale.CatalogSnapshotToken = command.CatalogSnapshotToken
		}
		sale.ReceiptReference = sale.ID
		sale.FiscalStatus = FiscalNotConfigured
		if command.CorrelationID != "" {
			sale.CorrelationID = command.CorrelationID
		} else {
			sale.CorrelationID = sale.ID
		}
		journalEntries := make([]finance.JournalEntry, 0, 2+len(command.Lines)*3)
		for _, requested := range command.Lines {
			var product catalog.Product
			var rate int64
			if command.Offline {
				product, rate, err = tx.OfflineCatalogProduct(ctx, command.Scope, command.CatalogSnapshotToken, requested.ProductID, command.ClientTimestamp.UTC())
			} else {
				product, err = tx.Product(ctx, command.Scope, requested.ProductID)
			}
			if err != nil {
				return err
			}
			if !product.Active {
				return ErrProductInactive
			}
			if product.ListPriceMinor <= 0 || product.StandardCostMinor < 0 ||
				product.RevenueAccountID == "" || product.COGSAccountID == "" || product.InventoryAccountID == "" {
				return ErrPostingConfig
			}
			if sale.Currency == "" {
				sale.Currency = product.Currency
			} else if sale.Currency != product.Currency {
				return ErrInvalidCommand
			}
			if !command.Offline {
				rate, err = tx.TaxRateBasisPoints(ctx, command.Scope, product.TaxCode, now)
				if err != nil {
					return err
				}
			}
			if command.Offline && rate != 0 {
				return ErrOfflineTaxUnsupported
			}
			available, err := tx.AvailableStock(ctx, command.Scope, product.ID)
			if err != nil {
				return err
			}
			if available < requested.Quantity {
				return fmt.Errorf("%w: product=%s available=%d requested=%d", ErrInsufficientStock, product.ID, available, requested.Quantity)
			}
			if command.Offline {
				allocated, err := tx.OfflineAllocation(ctx, command.Scope, command.DeviceID, product.ID)
				if err != nil {
					return err
				}
				if allocated < requested.Quantity {
					return fmt.Errorf("%w: product=%s allocated=%d requested=%d", devices.ErrAllocationExceeded, product.ID, allocated, requested.Quantity)
				}
			}
			lineSubtotal, err := checkedMultiply(product.ListPriceMinor, requested.Quantity)
			if err != nil {
				return err
			}
			lineTax, err := taxAmount(lineSubtotal, rate)
			if err != nil {
				return err
			}
			lineTotal, err := checkedAdd(lineSubtotal, lineTax)
			if err != nil {
				return err
			}
			lineCOGS, err := checkedMultiply(product.StandardCostMinor, requested.Quantity)
			if err != nil {
				return err
			}
			lineID, err := s.ids.New()
			if err != nil {
				return err
			}
			sale.Lines = append(sale.Lines, Line{
				ID: lineID, ProductID: product.ID, Quantity: requested.Quantity,
				UnitPriceMinor: product.ListPriceMinor, SubtotalMinor: lineSubtotal,
				TaxMinor: lineTax, TotalMinor: lineTotal,
				UnitCostMinor: product.StandardCostMinor, COGSMinor: lineCOGS,
			})
			if sale.SubtotalMinor, err = checkedAdd(sale.SubtotalMinor, lineSubtotal); err != nil {
				return err
			}
			if sale.TaxMinor, err = checkedAdd(sale.TaxMinor, lineTax); err != nil {
				return err
			}
			if sale.TotalMinor, err = checkedAdd(sale.TotalMinor, lineTotal); err != nil {
				return err
			}
			if sale.COGSMinor, err = checkedAdd(sale.COGSMinor, lineCOGS); err != nil {
				return err
			}

			movementID, err := s.ids.New()
			if err != nil {
				return err
			}
			if err := tx.AppendStockMovement(ctx, inventory.Movement{
				ID: movementID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID,
				BranchID: command.Scope.BranchID, WarehouseID: command.Scope.WarehouseID,
				ProductID: product.ID, SourceType: string(RecordSale), SourceID: sale.ID,
				Quantity: -requested.Quantity, OccurredAt: now,
			}); err != nil {
				return err
			}

			journalEntries = append(journalEntries,
				finance.JournalEntry{AccountID: product.RevenueAccountID, CreditMinor: lineSubtotal, Memo: "Product revenue"},
			)
			if lineCOGS > 0 {
				journalEntries = append(journalEntries,
					finance.JournalEntry{AccountID: product.COGSAccountID, DebitMinor: lineCOGS, Memo: "Cost of goods sold"},
					finance.JournalEntry{AccountID: product.InventoryAccountID, CreditMinor: lineCOGS, Memo: "Inventory issued"},
				)
			}
		}
		if len(sale.Currency) != 3 {
			return ErrInvalidCommand
		}
		if command.Offline {
			if enrolled.OfflineTransactionLimitMinor <= 0 || sale.TotalMinor > enrolled.OfflineTransactionLimitMinor {
				return devices.ErrOfflineLimit
			}
			location, err := time.LoadLocation(enrolled.TimeZone)
			if err != nil {
				return devices.ErrInvalidTimeZone
			}
			localCreatedAt := command.ClientTimestamp.UTC().In(location)
			startOfDay := time.Date(localCreatedAt.Year(), localCreatedAt.Month(), localCreatedAt.Day(), 0, 0, 0, 0, location)
			endOfDay := startOfDay.AddDate(0, 0, 1)
			dailyTotal, err := tx.OfflineSalesTotal(ctx, command.Scope, command.DeviceID, startOfDay.UTC(), endOfDay.UTC())
			if err != nil {
				return err
			}
			projected, err := checkedAdd(dailyTotal, sale.TotalMinor)
			if err != nil {
				return err
			}
			if enrolled.OfflineDailyLimitMinor <= 0 || projected > enrolled.OfflineDailyLimitMinor {
				return devices.ErrOfflineLimit
			}
		}
		if command.Kind == KindCredit {
			if err := tx.LockCustomerCredit(ctx, command.Scope, customer.ID); err != nil {
				return err
			}
			exposure, err := tx.CreditExposure(ctx, command.Scope, customer.ID)
			if err != nil {
				return err
			}
			newExposure, err := checkedAdd(exposure, sale.TotalMinor)
			if err != nil {
				return err
			}
			if newExposure > customer.CreditLimitMinor {
				return ErrCreditLimitExceeded
			}
			journalEntries = append([]finance.JournalEntry{{AccountID: posting.ReceivableAccountID, DebitMinor: sale.TotalMinor, Memo: "Customer receivable"}}, journalEntries...)
			entryID, err := s.ids.New()
			if err != nil {
				return err
			}
			if err := tx.AppendCustomerLedgerEntry(ctx, customers.LedgerEntry{
				ID: entryID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID,
				CustomerID: customer.ID, SourceType: string(RecordSale), SourceID: sale.ID,
				AmountMinor: sale.TotalMinor, Currency: sale.Currency, OccurredAt: now,
			}); err != nil {
				return err
			}
		} else {
			journalEntries = append([]finance.JournalEntry{{AccountID: cashAccount, DebitMinor: sale.TotalMinor, Memo: "Sale payment"}}, journalEntries...)
			paymentID, err := s.ids.New()
			if err != nil {
				return err
			}
			if err := tx.CreatePayment(ctx, Payment{
				ID: paymentID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID,
				SaleID: sale.ID, AccountID: cashAccount, Method: command.PaymentMethod,
				AmountMinor: sale.TotalMinor, Currency: sale.Currency, OccurredAt: now,
			}); err != nil {
				return err
			}
		}
		if sale.TaxMinor > 0 {
			journalEntries = append(journalEntries, finance.JournalEntry{AccountID: posting.TaxPayableAccountID, CreditMinor: sale.TaxMinor, Memo: "Output tax"})
		}
		journalID, err := s.ids.New()
		if err != nil {
			return err
		}
		journal := finance.Journal{
			ID: journalID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID,
			SourceType: string(RecordSale), SourceID: sale.ID, Currency: sale.Currency,
			OccurredAt: now, Entries: journalEntries,
		}
		if err := journal.Validate(); err != nil {
			return err
		}
		if err := tx.CreateSale(ctx, sale); err != nil {
			return err
		}
		if err := tx.CreateJournal(ctx, journal); err != nil {
			return err
		}
		if err := s.recordEvents(ctx, tx, sale, command.ActorID, "sale.posted", idempotencyKey, now); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, command.Scope, operation, idempotencyKey, sale.ID); err != nil {
			return err
		}
		result = sale
		return nil
	})
	if err != nil {
		return Sale{}, err
	}
	if err := ValidateWireSafeSale(result); err != nil {
		return Sale{}, err
	}
	return result, nil
}

func (s *Service) Reverse(ctx context.Context, command ReverseCommand) (Sale, error) {
	command.Scope = command.Scope.Normalize()
	command.SaleID = identity.NormalizeClaim(command.SaleID)
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.Reason = strings.TrimSpace(command.Reason)
	command.CorrelationID = identity.NormalizeClaim(command.CorrelationID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if err := command.Validate(); err != nil {
		return Sale{}, err
	}
	requestHash, err := hashCommand(command)
	if err != nil {
		return Sale{}, err
	}
	now := s.clock.Now().UTC()
	var result Sale
	err = s.repository.WithTransaction(ctx, func(tx Transaction) error {
		authorized, err := tx.Authorize(ctx, command.Scope, command.ActorID, "sales.reverse")
		if err != nil {
			return err
		}
		if !authorized {
			return ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, command.Scope, operationReverse, command.IdempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.Sale(ctx, command.Scope, resultID)
			return err
		}
		original, err := tx.Sale(ctx, command.Scope, command.SaleID)
		if err != nil {
			return err
		}
		open, err := tx.FiscalPeriodOpen(ctx, command.Scope, now)
		if err != nil {
			return err
		}
		if !open {
			return ErrFiscalPeriodClosed
		}
		if original.RecordType != RecordSale || original.Status == StatusReversed {
			return ErrAlreadyReversed
		}
		reversalID, err := s.ids.New()
		if err != nil {
			return err
		}
		reversal := Sale{
			ID: reversalID, Scope: original.Scope, RecordType: RecordReversal, Kind: original.Kind,
			Status: StatusPosted, CustomerID: original.CustomerID, Currency: original.Currency,
			SubtotalMinor: original.SubtotalMinor, TaxMinor: original.TaxMinor,
			TotalMinor: original.TotalMinor, COGSMinor: original.COGSMinor,
			PaymentMethod: original.PaymentMethod, ReversalOf: original.ID,
			ReversalReason: strings.TrimSpace(command.Reason), CreatedBy: command.ActorID,
			DocumentAt: now, ReceivedAt: now, AccountingAt: now,
			AccountingTimeBasis: AccountingTimeServerReceipt, CreatedAt: now,
		}
		reversal.ReceiptReference = reversal.ID
		reversal.FiscalStatus = FiscalNotConfigured
		if command.CorrelationID != "" {
			reversal.CorrelationID = command.CorrelationID
		} else {
			reversal.CorrelationID = reversal.ID
		}
		for _, line := range original.Lines {
			lineID, err := s.ids.New()
			if err != nil {
				return err
			}
			line.ID = lineID
			reversal.Lines = append(reversal.Lines, line)
		}
		movements, err := tx.StockMovementsBySource(ctx, command.Scope, string(RecordSale), original.ID)
		if err != nil {
			return err
		}
		if len(movements) == 0 {
			return ErrNotFound
		}
		for _, originalMovement := range movements {
			movementID, err := s.ids.New()
			if err != nil {
				return err
			}
			originalMovement.ID = movementID
			originalMovement.SourceType = string(RecordReversal)
			originalMovement.SourceID = reversal.ID
			originalMovement.Quantity = -originalMovement.Quantity
			originalMovement.OccurredAt = now
			if err := tx.AppendStockMovement(ctx, originalMovement); err != nil {
				return err
			}
		}
		originalJournal, err := tx.JournalBySource(ctx, command.Scope, string(RecordSale), original.ID)
		if err != nil {
			return err
		}
		journalID, err := s.ids.New()
		if err != nil {
			return err
		}
		reversalJournal := originalJournal.Reversed(journalID, string(RecordReversal), reversal.ID, now)
		if err := reversalJournal.Validate(); err != nil {
			return err
		}

		ledgerEntries, err := tx.CustomerLedgerBySource(ctx, command.Scope, string(RecordSale), original.ID)
		if err != nil {
			return err
		}
		for _, originalEntry := range ledgerEntries {
			entryID, err := s.ids.New()
			if err != nil {
				return err
			}
			originalEntry.ID = entryID
			originalEntry.SourceType = string(RecordReversal)
			originalEntry.SourceID = reversal.ID
			originalEntry.AmountMinor = -originalEntry.AmountMinor
			originalEntry.OccurredAt = now
			if err := tx.AppendCustomerLedgerEntry(ctx, originalEntry); err != nil {
				return err
			}
		}
		payments, err := tx.PaymentsBySale(ctx, command.Scope, original.ID)
		if err != nil {
			return err
		}
		for _, originalPayment := range payments {
			paymentID, err := s.ids.New()
			if err != nil {
				return err
			}
			originalPayment.ID = paymentID
			originalPayment.SaleID = reversal.ID
			originalPayment.AmountMinor = -originalPayment.AmountMinor
			originalPayment.OccurredAt = now
			if err := tx.CreatePayment(ctx, originalPayment); err != nil {
				return err
			}
		}
		if err := tx.CreateSale(ctx, reversal); err != nil {
			return err
		}
		if err := tx.CreateJournal(ctx, reversalJournal); err != nil {
			return err
		}
		if err := tx.MarkSaleReversed(ctx, command.Scope, original.ID, now); err != nil {
			return err
		}
		if err := s.recordEvents(ctx, tx, reversal, command.ActorID, "sale.reversed", original.ID, now); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, command.Scope, operationReverse, command.IdempotencyKey, reversal.ID); err != nil {
			return err
		}
		result = reversal
		return nil
	})
	if err != nil {
		return Sale{}, err
	}
	if err := ValidateWireSafeSale(result); err != nil {
		return Sale{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, scope tenancy.Scope, actorID, saleID string) (Sale, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	saleID = identity.NormalizeClaim(saleID)
	if scope.Validate() != nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(saleID) == "" {
		return Sale{}, ErrInvalidCommand
	}
	var result Sale
	err := s.repository.WithTransaction(ctx, func(tx Transaction) error {
		authorized, err := tx.Authorize(ctx, scope, actorID, "sales.read")
		if err != nil {
			return err
		}
		if !authorized {
			return ErrForbidden
		}
		result, err = tx.Sale(ctx, scope, saleID)
		return err
	})
	if err != nil {
		return Sale{}, err
	}
	if err := ValidateWireSafeSale(result); err != nil {
		return Sale{}, err
	}
	return result, nil
}

func normalizeCompleteCommand(command CompleteCommand) CompleteCommand {
	command.Scope = command.Scope.Normalize()
	command.CustomerID = identity.NormalizeClaim(command.CustomerID)
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.DeviceID = identity.NormalizeClaim(command.DeviceID)
	command.ClientTransactionID = identity.NormalizeClaim(command.ClientTransactionID)
	command.CorrelationID = identity.NormalizeClaim(command.CorrelationID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.PaymentMethod = strings.ToUpper(strings.TrimSpace(command.PaymentMethod))
	command.AppVersion = strings.TrimSpace(command.AppVersion)
	command.CatalogSnapshotToken = identity.NormalizeClaim(command.CatalogSnapshotToken)
	if !command.ClientTimestamp.IsZero() {
		command.ClientTimestamp = command.ClientTimestamp.UTC()
	}
	command.Lines = append([]CommandLine(nil), command.Lines...)
	for index := range command.Lines {
		command.Lines[index].ProductID = identity.NormalizeClaim(command.Lines[index].ProductID)
	}
	sort.Slice(command.Lines, func(left, right int) bool {
		return command.Lines[left].ProductID < command.Lines[right].ProductID
	})
	return command
}

func (s *Service) recordEvents(ctx context.Context, tx Transaction, sale Sale, actorID, eventType, causationID string, now time.Time) error {
	payload, err := json.Marshal(sale)
	if err != nil {
		return fmt.Errorf("marshal sale event: %w", err)
	}
	auditID, err := s.ids.New()
	if err != nil {
		return err
	}
	if err := tx.AppendAuditEvent(ctx, audit.Event{
		ID: auditID, TenantID: sale.Scope.TenantID, CompanyID: sale.Scope.CompanyID,
		ActorID: actorID, Action: eventType, EntityType: "sale", EntityID: sale.ID,
		CorrelationID: sale.CorrelationID, CausationID: causationID,
		Data: payload, OccurredAt: now,
	}); err != nil {
		return err
	}
	outboxID, err := s.ids.New()
	if err != nil {
		return err
	}
	return tx.AppendOutboxEvent(ctx, outbox.Event{
		ID: outboxID, TenantID: sale.Scope.TenantID, CompanyID: sale.Scope.CompanyID,
		AggregateType: "sale", AggregateID: sale.ID, EventType: eventType,
		Version: 1, CorrelationID: sale.CorrelationID, CausationID: causationID,
		Payload: payload, OccurredAt: now,
	})
}

func hashCommand(command any) (string, error) {
	encoded, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("hash command: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func checkedMultiply(left, right int64) (int64, error) {
	if left < 0 || right < 0 || left > MaxWireSafeInteger || right > MaxWireSafeInteger || (right != 0 && left > MaxWireSafeInteger/right) {
		if left >= 0 && right >= 0 {
			return 0, ErrUnsafeWireInteger
		}
		return 0, ErrMoneyOverflow
	}
	return left * right, nil
}

func checkedAdd(left, right int64) (int64, error) {
	if left < 0 || right < 0 {
		return 0, ErrMoneyOverflow
	}
	if left > MaxWireSafeInteger || right > MaxWireSafeInteger || left > MaxWireSafeInteger-right {
		return 0, ErrUnsafeWireInteger
	}
	return left + right, nil
}

func taxAmount(subtotal, basisPoints int64) (int64, error) {
	if basisPoints < 0 || basisPoints > 10000 {
		return 0, ErrInvalidCommand
	}
	if subtotal < 0 || subtotal > MaxWireSafeInteger {
		return 0, ErrUnsafeWireInteger
	}
	whole, remainder := subtotal/10_000, subtotal%10_000
	wholeTax, err := checkedMultiply(whole, basisPoints)
	if err != nil {
		return 0, err
	}
	remainderTax := (remainder*basisPoints + 5_000) / 10_000
	result, err := checkedAdd(wholeTax, remainderTax)
	if err != nil {
		return 0, err
	}
	return result, nil
}
