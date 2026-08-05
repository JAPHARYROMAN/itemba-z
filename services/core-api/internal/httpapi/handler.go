// Package httpapi exposes the versioned REST boundary for the core application.
// Authentication is delegated to an explicit adapter that returns a verified
// actor and organizational scope. Production never derives identity from X-* headers.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

const maxBodyBytes = 1 << 20

var correlationIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Handler struct {
	sales           *sales.Service
	read            *readmodel.Service
	mobile          *mobile.Service
	receivables     *receivables.Service
	operations      *operations.Service
	banking         *banking.Service
	financialops    *financialops.Service
	reporting       *reporting.Service
	advancedfinance *advancedfinance.Service
	treasury        *treasury.Service
	groupfinance    *groupfinance.Service
	logger          *slog.Logger
	authenticator   Authenticator
}

func New(salesService *sales.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	if salesService == nil {
		return nil, errors.New("sales service is required")
	}
	if authenticator == nil {
		return nil, errors.New("authenticator is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{sales: salesService, logger: logger, authenticator: authenticator}, nil
}

func NewLive(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := New(salesService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if readService == nil || mobileService == nil || receivablesService == nil {
		return nil, errors.New("read, mobile, and receivables services are required")
	}
	handler.read, handler.mobile, handler.receivables = readService, mobileService, receivablesService
	return handler, nil
}

func NewLiveWithOperations(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLive(salesService, readService, mobileService, receivablesService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if operationsService == nil {
		return nil, errors.New("operations service is required")
	}
	handler.operations = operationsService
	return handler, nil
}

func NewLiveWithModules(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, bankingService *banking.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLiveWithOperations(salesService, readService, mobileService, receivablesService, operationsService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if bankingService == nil {
		return nil, errors.New("banking service is required")
	}
	handler.banking = bankingService
	return handler, nil
}

func NewLiveWithFinance(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, bankingService *banking.Service, financialService *financialops.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLiveWithModules(salesService, readService, mobileService, receivablesService, operationsService, bankingService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if financialService == nil {
		return nil, errors.New("financial operations service is required")
	}
	handler.financialops = financialService
	return handler, nil
}

func NewLiveWithReporting(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, bankingService *banking.Service, financialService *financialops.Service, reportingService *reporting.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLiveWithFinance(salesService, readService, mobileService, receivablesService, operationsService, bankingService, financialService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if reportingService == nil {
		return nil, errors.New("financial reporting service is required")
	}
	handler.reporting = reportingService
	return handler, nil
}

func NewLiveWithAdvancedFinance(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, bankingService *banking.Service, financialService *financialops.Service, reportingService *reporting.Service, advancedService *advancedfinance.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLiveWithReporting(salesService, readService, mobileService, receivablesService, operationsService, bankingService, financialService, reportingService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if advancedService == nil {
		return nil, errors.New("advanced finance service is required")
	}
	handler.advancedfinance = advancedService
	return handler, nil
}

func NewLiveWithTreasury(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, bankingService *banking.Service, financialService *financialops.Service, reportingService *reporting.Service, advancedService *advancedfinance.Service, treasuryService *treasury.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLiveWithAdvancedFinance(salesService, readService, mobileService, receivablesService, operationsService, bankingService, financialService, reportingService, advancedService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if treasuryService == nil {
		return nil, errors.New("treasury service is required")
	}
	handler.treasury = treasuryService
	return handler, nil
}

func NewLiveWithGroupFinance(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, receivablesService *receivables.Service, operationsService *operations.Service, bankingService *banking.Service, financialService *financialops.Service, reportingService *reporting.Service, advancedService *advancedfinance.Service, treasuryService *treasury.Service, groupService *groupfinance.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := NewLiveWithTreasury(salesService, readService, mobileService, receivablesService, operationsService, bankingService, financialService, reportingService, advancedService, treasuryService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if groupService == nil {
		return nil, errors.New("group finance service is required")
	}
	handler.groupfinance = groupService
	return handler, nil
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.health)
	mux.HandleFunc("POST /v1/sales", h.completeSale)
	if h.operations != nil {
		mux.HandleFunc("GET /v1/operations/documents", h.listOperationDocuments)
		mux.HandleFunc("POST /v1/operations/documents", h.createOperationDocument)
		mux.HandleFunc("GET /v1/operations/documents/{documentID}", h.getOperationDocument)
		mux.HandleFunc("POST /v1/operations/documents/{documentID}/transitions", h.transitionOperationDocument)
		mux.HandleFunc("GET /v1/suppliers", h.listSuppliers)
	}
	if h.banking != nil {
		mux.HandleFunc("GET /v1/banking/accounts", h.listBankAccounts)
		mux.HandleFunc("GET /v1/banking/statements", h.listBankStatements)
		mux.HandleFunc("POST /v1/banking/statements", h.importBankStatement)
		mux.HandleFunc("GET /v1/banking/statements/{statementID}", h.getBankStatement)
		mux.HandleFunc("POST /v1/banking/statements/{statementID}/lines/{lineID}/matches", h.matchBankStatementLine)
		mux.HandleFunc("POST /v1/banking/statements/{statementID}/reconciliation", h.reconcileBankStatement)
	}
	if h.financialops != nil {
		mux.HandleFunc("GET /v1/finance/documents", h.listFinancialDocuments)
		mux.HandleFunc("POST /v1/finance/documents", h.createFinancialDocument)
		mux.HandleFunc("GET /v1/finance/documents/{documentID}", h.getFinancialDocument)
		mux.HandleFunc("POST /v1/finance/documents/{documentID}/transitions", h.transitionFinancialDocument)
		mux.HandleFunc("GET /v1/finance/fiscal-periods", h.listFiscalPeriods)
		mux.HandleFunc("GET /v1/finance/fiscal-period-actions", h.listFiscalPeriodActions)
		mux.HandleFunc("POST /v1/finance/fiscal-periods/{periodID}/actions", h.requestFiscalPeriodAction)
		mux.HandleFunc("POST /v1/finance/fiscal-period-actions/{actionID}/approval", h.approveFiscalPeriodAction)
		mux.HandleFunc("GET /v1/finance/accounts", h.listGLAccounts)
		mux.HandleFunc("POST /v1/finance/accounts", h.createGLAccount)
		mux.HandleFunc("POST /v1/finance/accounts/{accountID}/decisions", h.decideGLAccount)
		mux.HandleFunc("GET /v1/finance/posting-mappings", h.listPostingMappings)
		mux.HandleFunc("POST /v1/finance/posting-mappings", h.createPostingMapping)
		mux.HandleFunc("POST /v1/finance/posting-mappings/{mappingID}/decisions", h.decidePostingMapping)
	}
	if h.read != nil {
		mux.HandleFunc("GET /v1/context", h.workingContext)
		mux.HandleFunc("GET /v1/customers", h.listCustomers)
		mux.HandleFunc("GET /v1/products", h.listProducts)
		mux.HandleFunc("GET /v1/sales", h.listSales)
	}
	if h.reporting != nil {
		mux.HandleFunc("GET /v1/reports/financial/trial-balance", h.trialBalance)
		mux.HandleFunc("GET /v1/reports/financial/general-ledger", h.generalLedger)
		mux.HandleFunc("GET /v1/reports/financial/profit-and-loss", h.profitAndLoss)
		mux.HandleFunc("GET /v1/reports/financial/balance-sheet", h.balanceSheet)
		mux.HandleFunc("GET /v1/reports/financial/cash-flow", h.cashFlow)
		mux.HandleFunc("POST /v1/reports/financial/exports", h.exportFinancialReport)
	}
	if h.advancedfinance != nil {
		mux.HandleFunc("GET /v1/finance/budgets", h.listBudgets)
		mux.HandleFunc("POST /v1/finance/budgets", h.createBudget)
		mux.HandleFunc("POST /v1/finance/budgets/{budgetID}/transitions", h.transitionBudget)
		mux.HandleFunc("GET /v1/finance/budgets/{budgetID}/actual", h.budgetActual)
		mux.HandleFunc("GET /v1/finance/assets", h.listAssets)
		mux.HandleFunc("POST /v1/finance/assets", h.createAsset)
		mux.HandleFunc("POST /v1/finance/assets/from-purchase", h.createPurchasedAsset)
		mux.HandleFunc("POST /v1/finance/assets/{assetID}/transitions", h.transitionAsset)
		mux.HandleFunc("POST /v1/finance/assets/{assetID}/depreciation", h.depreciateAsset)
		mux.HandleFunc("POST /v1/finance/assets/{assetID}/disposal", h.disposeAsset)
	}
	if h.treasury != nil {
		mux.HandleFunc("GET /v1/finance/facilities", h.listTreasuryFacilities)
		mux.HandleFunc("POST /v1/finance/facilities", h.createTreasuryFacility)
		mux.HandleFunc("POST /v1/finance/facilities/{facilityID}/transitions", h.transitionTreasuryFacility)
		mux.HandleFunc("POST /v1/finance/facilities/{facilityID}/transactions", h.postTreasuryTransaction)
	}
	if h.groupfinance != nil {
		mux.HandleFunc("GET /v1/finance/intercompany", h.listIntercompany)
		mux.HandleFunc("POST /v1/finance/intercompany", h.createIntercompany)
		mux.HandleFunc("POST /v1/finance/intercompany/{transactionID}/transitions", h.transitionIntercompany)
		mux.HandleFunc("GET /v1/reports/financial/consolidation", h.consolidation)
	}
	if h.mobile != nil {
		mux.HandleFunc("POST /v1/mobile/devices/enroll", h.enrollDevice)
		mux.HandleFunc("GET /v1/mobile/devices", h.listManagedDevices)
		mux.HandleFunc("POST /v1/mobile/devices/{deviceID}/status-changes", h.changeManagedDeviceStatus)
		mux.HandleFunc("POST /v1/mobile/devices/{deviceID}/allocation-changes", h.changeManagedDeviceAllocation)
		mux.HandleFunc("POST /v1/mobile/sync/sales", h.syncMobileSale)
		mux.HandleFunc("GET /v1/mobile/reconciliation-cases", h.listMobileReconciliationCases)
		mux.HandleFunc("GET /v1/mobile/reconciliation-cases/{caseID}", h.getMobileReconciliationCase)
		mux.HandleFunc("POST /v1/mobile/reconciliation-cases/{caseID}/resolutions", h.resolveMobileReconciliationCase)
	}
	mux.HandleFunc("GET /v1/customers/{customerID}/account", h.customerAccount)
	mux.HandleFunc("POST /v1/customers/{customerID}/credit-policies", h.scheduleCustomerCreditPolicy)
	mux.HandleFunc("POST /v1/customers/{customerID}/collections", h.receiveCustomerCollection)
	mux.HandleFunc("GET /v1/sales/{saleID}", h.getSale)
	mux.HandleFunc("POST /v1/sales/{saleID}/reversals", h.reverseSale)
	return securityHeaders(correlationIDs(identity.UUIDGenerator{}, h.observe(mux)))
}

type receiveCustomerCollectionRequest struct {
	InvoiceSaleID string `json:"invoice_sale_id"`
	Method        string `json:"method"`
	AmountMinor   int64  `json:"amount_minor"`
	Currency      string `json:"currency"`
}

type importBankStatementLineRequest struct {
	TransactionAt     string `json:"transaction_at"`
	ExternalReference string `json:"external_reference"`
	Description       string `json:"description"`
	AmountMinor       int64  `json:"amount_minor"`
}
type importBankStatementRequest struct {
	AccountID         string                           `json:"account_id"`
	ExternalReference string                           `json:"external_reference"`
	Currency          string                           `json:"currency"`
	PeriodStart       string                           `json:"period_start"`
	PeriodEnd         string                           `json:"period_end"`
	OpeningMinor      int64                            `json:"opening_minor"`
	ClosingMinor      int64                            `json:"closing_minor"`
	Lines             []importBankStatementLineRequest `json:"lines"`
}

func (h *Handler) listBankAccounts(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	result, err := h.banking.Accounts(request.Context(), principal.Scope, principal.ActorID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": result})
}
func (h *Handler) listBankStatements(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	limit, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.banking.Statements(request.Context(), principal.Scope, principal.ActorID, request.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}
func (h *Handler) getBankStatement(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	id, err := identity.CanonicalUUID(request.PathValue("statementID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "statementID must be a UUID")
		return
	}
	result, err := h.banking.Statement(request.Context(), principal.Scope, principal.ActorID, id)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}
func (h *Handler) importBankStatement(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	var body importBankStatementRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	accountID, err := identity.CanonicalUUID(body.AccountID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "account_id must be a UUID")
		return
	}
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(body.PeriodStart))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "period_start must be RFC 3339")
		return
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(body.PeriodEnd))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "period_end must be RFC 3339")
		return
	}
	command := banking.ImportCommand{Scope: principal.Scope, AccountID: accountID, ExternalReference: body.ExternalReference, Currency: body.Currency, PeriodStart: start, PeriodEnd: end, OpeningMinor: body.OpeningMinor, ClosingMinor: body.ClosingMinor, ActorID: principal.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(writer)}
	for _, line := range body.Lines {
		at, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(line.TransactionAt))
		if parseErr != nil {
			writeProblem(writer, http.StatusBadRequest, "invalid_request", "line transaction_at must be RFC 3339")
			return
		}
		command.Lines = append(command.Lines, banking.ImportLine{TransactionAt: at, ExternalReference: line.ExternalReference, Description: line.Description, AmountMinor: line.AmountMinor})
	}
	result, err := h.banking.Import(request.Context(), command)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/v1/banking/statements/"+result.ID)
	writeJSON(writer, http.StatusCreated, result)
}

type matchBankStatementRequest struct {
	JournalLineID string `json:"journal_line_id"`
	Reason        string `json:"reason"`
}

func (h *Handler) matchBankStatementLine(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	statementID, err := identity.CanonicalUUID(request.PathValue("statementID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "statementID must be a UUID")
		return
	}
	lineID, err := identity.CanonicalUUID(request.PathValue("lineID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "lineID must be a UUID")
		return
	}
	var body matchBankStatementRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.banking.Match(request.Context(), banking.MatchCommand{Scope: principal.Scope, StatementID: statementID, LineID: lineID, JournalLineID: body.JournalLineID, Reason: body.Reason, ActorID: principal.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(writer)})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

type reconcileBankStatementRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) reconcileBankStatement(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	statementID, err := identity.CanonicalUUID(request.PathValue("statementID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "statementID must be a UUID")
		return
	}
	var body reconcileBankStatementRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.banking.Reconcile(request.Context(), banking.ReconcileCommand{Scope: principal.Scope, StatementID: statementID, Reason: body.Reason, ActorID: principal.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(writer)})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (h *Handler) receiveCustomerCollection(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	customerID, err := identity.CanonicalUUID(request.PathValue("customerID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "customerID must be a UUID")
		return
	}
	var body receiveCustomerCollectionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.receivables.ReceiveCollection(request.Context(), receivables.ReceiveCollectionCommand{Scope: principal.Scope, ActorID: principal.ActorID, CustomerID: customerID, InvoiceSaleID: body.InvoiceSaleID, Method: body.Method, AmountMinor: body.AmountMinor, Currency: body.Currency, IdempotencyKey: idem, CorrelationID: correlationID(writer)})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (h *Handler) listSuppliers(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	limit, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.operations.Suppliers(request.Context(), principal.Scope, principal.ActorID, request.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

type operationLineRequest struct {
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
}
type createOperationRequest struct {
	Type                   operations.DocumentType `json:"type"`
	PartyType              operations.PartyType    `json:"party_type"`
	PartyID                string                  `json:"party_id,omitempty"`
	SourceDocumentID       string                  `json:"source_document_id,omitempty"`
	DestinationWarehouseID string                  `json:"destination_warehouse_id,omitempty"`
	Currency               string                  `json:"currency"`
	Reason                 string                  `json:"reason"`
	Lines                  []operationLineRequest  `json:"lines"`
}

func (h *Handler) createOperationDocument(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	var body createOperationRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	command := operations.CreateCommand{Scope: principal.Scope, Type: body.Type, PartyType: body.PartyType, PartyID: body.PartyID,
		SourceDocumentID: body.SourceDocumentID, DestinationWarehouseID: body.DestinationWarehouseID, Currency: body.Currency,
		Reason: body.Reason, ActorID: principal.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(writer)}
	for _, line := range body.Lines {
		command.Lines = append(command.Lines, operations.CommandLine{ProductID: line.ProductID, Quantity: line.Quantity, UnitPriceMinor: line.UnitPriceMinor})
	}
	result, err := h.operations.Create(request.Context(), command)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/v1/operations/documents/"+result.ID)
	writeJSON(writer, http.StatusCreated, result)
}

func (h *Handler) listOperationDocuments(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	limit, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.operations.List(request.Context(), principal.Scope, principal.ActorID, operations.DocumentType(request.URL.Query().Get("type")), request.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) getOperationDocument(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	id, err := identity.CanonicalUUID(request.PathValue("documentID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "documentID must be a UUID")
		return
	}
	result, err := h.operations.Get(request.Context(), principal.Scope, principal.ActorID, id)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

type transitionOperationRequest struct {
	Status        operations.Status `json:"status"`
	Reason        string            `json:"reason"`
	PaymentMethod string            `json:"payment_method,omitempty"`
}

func (h *Handler) transitionOperationDocument(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	id, err := identity.CanonicalUUID(request.PathValue("documentID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "documentID must be a UUID")
		return
	}
	var body transitionOperationRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.operations.Transition(request.Context(), operations.TransitionCommand{Scope: principal.Scope, DocumentID: id, ToStatus: body.Status, Reason: body.Reason, PaymentMethod: body.PaymentMethod, ActorID: principal.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(writer)})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) customerAccount(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	customerID, err := identity.CanonicalUUID(request.PathValue("customerID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "customerID must be a UUID")
		return
	}
	result, err := h.receivables.Account(request.Context(), principal.Scope, principal.ActorID, customerID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

type scheduleCustomerCreditPolicyRequest struct {
	CreditEnabled    bool   `json:"credit_enabled"`
	CreditLimitMinor int64  `json:"credit_limit_minor"`
	PaymentTermsDays int64  `json:"payment_terms_days"`
	MaxOverdueDays   int64  `json:"max_overdue_days"`
	RiskStatus       string `json:"risk_status"`
	Reason           string `json:"reason"`
	EffectiveFrom    string `json:"effective_from"`
}

func (h *Handler) scheduleCustomerCreditPolicy(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	customerID, err := identity.CanonicalUUID(request.PathValue("customerID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "customerID must be a UUID")
		return
	}
	var body scheduleCustomerCreditPolicyRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	effectiveFrom, err := time.Parse(time.RFC3339, strings.TrimSpace(body.EffectiveFrom))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "effective_from must be an RFC 3339 timestamp")
		return
	}
	result, err := h.receivables.ScheduleCreditPolicy(request.Context(), receivables.ScheduleCreditPolicyCommand{
		Scope: principal.Scope, ActorID: principal.ActorID, CustomerID: customerID,
		CreditEnabled: body.CreditEnabled, CreditLimitMinor: body.CreditLimitMinor,
		PaymentTermsDays: body.PaymentTermsDays, MaxOverdueDays: body.MaxOverdueDays,
		RiskStatus: customers.CreditRiskStatus(body.RiskStatus), Reason: body.Reason, EffectiveFrom: effectiveFrom,
		IdempotencyKey: idempotencyKey, CorrelationID: correlationID(writer),
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (h *Handler) listManagedDevices(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.mobile.ManagedDevices(request.Context(), principal.Scope, principal.ActorID, request.URL.Query().Get("cursor"), pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

type changeManagedDeviceStatusRequest struct {
	Status devices.Status `json:"status"`
	Reason string         `json:"reason"`
}

func (h *Handler) changeManagedDeviceStatus(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	deviceID, err := identity.CanonicalUUID(request.PathValue("deviceID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "deviceID must be a UUID")
		return
	}
	var body changeManagedDeviceStatusRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.mobile.ChangeDeviceStatus(request.Context(), mobile.ChangeDeviceStatusCommand{
		Scope: principal.Scope, ActorID: principal.ActorID, DeviceID: deviceID,
		Status: body.Status, Reason: body.Reason, IdempotencyKey: idempotencyKey,
		CorrelationID: correlationID(writer),
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

type changeManagedDeviceAllocationRequest struct {
	ProductID         string `json:"product_id"`
	AllocatedQuantity int64  `json:"allocated_quantity"`
	Reason            string `json:"reason"`
}

func (h *Handler) changeManagedDeviceAllocation(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey, ok := requireIdempotencyKey(writer, request)
	if !ok {
		return
	}
	deviceID, err := identity.CanonicalUUID(request.PathValue("deviceID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "deviceID must be a UUID")
		return
	}
	var body changeManagedDeviceAllocationRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	productID, err := identity.CanonicalUUID(body.ProductID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "product_id must be a UUID")
		return
	}
	result, err := h.mobile.ChangeDeviceAllocation(request.Context(), mobile.ChangeDeviceAllocationCommand{
		Scope: principal.Scope, ActorID: principal.ActorID, DeviceID: deviceID, ProductID: productID,
		AllocatedQuantity: body.AllocatedQuantity, Reason: body.Reason,
		IdempotencyKey: idempotencyKey, CorrelationID: correlationID(writer),
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func requireIdempotencyKey(writer http.ResponseWriter, request *http.Request) (string, bool) {
	value := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(value) < 16 || len(value) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return "", false
	}
	return value, true
}

func (h *Handler) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "healthy", "version": "dev"})
}

func (h *Handler) workingContext(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	result, err := h.read.Context(request.Context(), principal.Scope, principal.ActorID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) listCustomers(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	eligible, ok := parseOptionalBool(writer, request, "credit_eligible")
	if !ok {
		return
	}
	result, err := h.read.Customers(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("query"), request.URL.Query().Get("cursor"),
		request.URL.Query().Get("snapshot_token"), eligible, pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) listProducts(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.read.Products(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("query"), request.URL.Query().Get("cursor"),
		request.URL.Query().Get("snapshot_token"), pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) listSales(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.read.Sales(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("cursor"), pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, presentSalePage(result))
}

type enrollDeviceRequest struct {
	DeviceID                      string  `json:"device_id"`
	DeviceName                    string  `json:"device_name"`
	AppVersion                    string  `json:"app_version"`
	InstalledMasterDataVersion    *int64  `json:"installed_master_data_version,omitempty"`
	InstalledPriceVersion         *int64  `json:"installed_price_version,omitempty"`
	InstalledCatalogSnapshotToken *string `json:"installed_catalog_snapshot_token,omitempty"`
}

func (h *Handler) enrollDevice(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	var body enrollDeviceRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	deviceID, err := identity.CanonicalUUID(body.DeviceID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "device_id must be a UUID")
		return
	}
	result, err := h.mobile.Enroll(request.Context(), mobile.EnrollCommand{
		Scope: principal.Scope, ActorID: principal.ActorID, DeviceID: deviceID,
		DeviceName: body.DeviceName, AppVersion: body.AppVersion,
		InstalledMasterDataVersion: body.InstalledMasterDataVersion, InstalledPriceVersion: body.InstalledPriceVersion,
		InstalledCatalogSnapshotToken: body.InstalledCatalogSnapshotToken,
		CorrelationID:                 correlationID(writer),
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) syncMobileSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	var body mobile.SyncCommand
	if !decodeJSON(writer, request, &body) {
		return
	}
	if !canonicalizeMobileCommand(&body) {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "device_id and client_transaction_id must be UUIDs")
		return
	}
	result, err := h.mobile.SyncSale(request.Context(), principal.Scope, principal.ActorID, correlationID(writer), body)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, presentMobileSync(result))
}

func (h *Handler) listMobileReconciliationCases(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.mobile.ReconciliationCases(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("status"), request.URL.Query().Get("cursor"), pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) getMobileReconciliationCase(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	caseID, err := identity.CanonicalUUID(request.PathValue("caseID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "caseID must be a UUID")
		return
	}
	result, err := h.mobile.ReconciliationCase(request.Context(), principal.Scope, principal.ActorID, caseID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

type resolveMobileReconciliationRequest struct {
	Action            mobile.ResolutionAction `json:"action"`
	Reason            string                  `json:"reason"`
	ExternalReference string                  `json:"external_reference,omitempty"`
}

func (h *Handler) resolveMobileReconciliationCase(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return
	}
	caseID, err := identity.CanonicalUUID(request.PathValue("caseID"))
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "caseID must be a UUID")
		return
	}
	var body resolveMobileReconciliationRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.mobile.ResolveReconciliation(request.Context(), mobile.ResolveReconciliationCommand{
		Scope: principal.Scope, ActorID: principal.ActorID, CaseID: caseID,
		Action: body.Action, Reason: body.Reason, ExternalReference: body.ExternalReference,
		IdempotencyKey: idempotencyKey, CorrelationID: correlationID(writer),
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/v1/mobile/reconciliation-cases/"+caseID)
	writeJSON(writer, http.StatusCreated, result)
}

type completeSaleRequest struct {
	CustomerID       string              `json:"customer_id"`
	SourceDocumentID string              `json:"source_document_id,omitempty"`
	Kind             sales.Kind          `json:"kind"`
	PaymentMethod    string              `json:"payment_method,omitempty"`
	Lines            []sales.CommandLine `json:"lines"`
}

func (h *Handler) completeSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return
	}
	var body completeSaleRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	if !canonicalizeSaleClaims(&body.CustomerID, body.Lines) {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "customer_id and product_id values must be UUIDs")
		return
	}
	result, err := h.sales.Complete(request.Context(), sales.CompleteCommand{
		Scope: principal.Scope, CustomerID: body.CustomerID, Kind: body.Kind,
		SourceDocumentID: body.SourceDocumentID,
		PaymentMethod:    body.PaymentMethod, Lines: body.Lines,
		ActorID: principal.ActorID, CorrelationID: correlationID(writer), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/v1/sales/"+result.ID)
	writeJSON(writer, http.StatusCreated, presentSale(result))
}

func (h *Handler) getSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	saleID, canonicalErr := identity.CanonicalUUID(request.PathValue("saleID"))
	if canonicalErr != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "sale_id must be a UUID")
		return
	}
	result, err := h.sales.Get(request.Context(), principal.Scope, principal.ActorID, saleID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, presentSale(result))
}

type reverseSaleRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) reverseSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return
	}
	var body reverseSaleRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	saleID, canonicalErr := identity.CanonicalUUID(request.PathValue("saleID"))
	if canonicalErr != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "sale_id must be a UUID")
		return
	}
	result, err := h.sales.Reverse(request.Context(), sales.ReverseCommand{
		Scope: principal.Scope, SaleID: saleID, Reason: body.Reason,
		ActorID: principal.ActorID, CorrelationID: correlationID(writer), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, presentSale(result))
}

func (h *Handler) requestContext(writer http.ResponseWriter, request *http.Request) (Principal, bool) {
	principal, err := h.authenticator.Authenticate(request.Context(), request)
	if err == nil {
		principal, err = principal.canonicalized()
	}
	if err != nil {
		writeProblem(writer, http.StatusUnauthorized, "invalid_security_context", "verified actor and organizational scope are required")
		return Principal{}, false
	}
	return principal, true
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, destination any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with known fields")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeProblem(writer, http.StatusBadRequest, "invalid_json", "request body must contain one JSON value")
		return false
	}
	return true
}

func canonicalizeSaleClaims(customerID *string, lines []sales.CommandLine) bool {
	canonical, err := identity.CanonicalUUID(*customerID)
	if err != nil {
		return false
	}
	*customerID = canonical
	for index := range lines {
		canonical, err := identity.CanonicalUUID(lines[index].ProductID)
		if err != nil {
			return false
		}
		lines[index].ProductID = canonical
	}
	return true
}

func canonicalizeMobileCommand(command *mobile.SyncCommand) bool {
	deviceID, err := identity.CanonicalUUID(command.DeviceID)
	if err != nil {
		return false
	}
	clientID, err := identity.CanonicalUUID(command.ClientTransactionID)
	snapshotToken, snapshotErr := identity.CanonicalUUID(command.CatalogSnapshotToken)
	if err != nil || snapshotErr != nil || !canonicalizeSaleClaims(&command.CustomerID, command.Lines) {
		return false
	}
	command.DeviceID, command.ClientTransactionID, command.CatalogSnapshotToken = deviceID, clientID, snapshotToken
	return true
}

func parsePageSize(writer http.ResponseWriter, request *http.Request) (int, bool) {
	value := strings.TrimSpace(request.URL.Query().Get("page_size"))
	if value == "" {
		return 50, true
	}
	pageSize, err := strconv.Atoi(value)
	if err != nil || pageSize < 1 || pageSize > 200 {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "page_size must be between 1 and 200")
		return 0, false
	}
	return pageSize, true
}

func parseOptionalBool(writer http.ResponseWriter, request *http.Request, name string) (*bool, bool) {
	value := strings.TrimSpace(request.URL.Query().Get(name))
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", name+" must be true or false")
		return nil, false
	}
	return &parsed, true
}

func (h *Handler) writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, sales.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, sales.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, devices.ErrNotEnrolled), errors.Is(err, devices.ErrScopeMismatch):
		status, code = http.StatusForbidden, "device_binding_forbidden"
	case errors.Is(err, sales.ErrIdempotencyConflict):
		status, code = http.StatusConflict, "idempotency_conflict"
	case errors.Is(err, customers.ErrCreditPolicySequence), errors.Is(err, customers.ErrCreditPolicyBackdated):
		status, code = http.StatusConflict, "credit_policy_conflict"
	case errors.Is(err, customers.ErrReceivablesUnbalanced):
		status, code = http.StatusConflict, "receivables_reconciliation_required"
	case errors.Is(err, mobile.ErrReconciliationResolved):
		status, code = http.StatusConflict, "reconciliation_already_resolved"
	case errors.Is(err, devices.ErrStatusUnchanged):
		status, code = http.StatusConflict, "device_status_unchanged"
	case errors.Is(err, devices.ErrAllocationBelowConsumed), errors.Is(err, devices.ErrAllocationOvercommitted):
		status, code = http.StatusConflict, "allocation_conflict"
	case errors.Is(err, sales.ErrInsufficientStock):
		status, code = http.StatusConflict, "insufficient_stock"
	case errors.Is(err, operations.ErrInsufficientStock), errors.Is(err, operations.ErrOverReceipt), errors.Is(err, operations.ErrOverInvoice), errors.Is(err, operations.ErrPayableExceeded), errors.Is(err, operations.ErrInvalidTransition), errors.Is(err, operations.ErrSeparationOfDuties):
		status, code = http.StatusConflict, "operations_conflict"
	case errors.Is(err, banking.ErrStatementImbalance), errors.Is(err, banking.ErrDuplicateStatement), errors.Is(err, banking.ErrMatchMismatch), errors.Is(err, banking.ErrAlreadyMatched), errors.Is(err, banking.ErrUnmatchedLines), errors.Is(err, banking.ErrSeparationOfDuties), errors.Is(err, banking.ErrAlreadyReconciled):
		status, code = http.StatusConflict, "bank_reconciliation_conflict"
	case errors.Is(err, banking.ErrInactiveAccount):
		status, code = http.StatusUnprocessableEntity, "business_rule_violation"
	case errors.Is(err, financialops.ErrInvalidTransition), errors.Is(err, financialops.ErrSeparationOfDuties), errors.Is(err, financialops.ErrPeriodCloseBlocked), errors.Is(err, financialops.ErrAlreadyReversed), errors.Is(err, financialops.ErrAccountGovernance):
		status, code = http.StatusConflict, "financial_control_conflict"
	case errors.Is(err, financialops.ErrPeriodClosed):
		status, code = http.StatusConflict, "fiscal_period_closed"
	case errors.Is(err, sales.ErrFiscalPeriodClosed):
		status, code = http.StatusConflict, "fiscal_period_closed"
	case errors.Is(err, sales.ErrAlreadyReversed):
		status, code = http.StatusConflict, "already_reversed"
	case errors.Is(err, sales.ErrUnsafeWireInteger):
		status, code = http.StatusUnprocessableEntity, "wire_integer_out_of_range"
	case errors.Is(err, sales.ErrOfflineReconciliation),
		errors.Is(err, sales.ErrOfflinePeriodReconciliation),
		errors.Is(err, sales.ErrOfflineClockReconciliation):
		status, code = http.StatusConflict, "offline_reconciliation_required"
	case errors.Is(err, sales.ErrGeneralCustomerCredit), errors.Is(err, sales.ErrCustomerCreditDisabled), errors.Is(err, sales.ErrCreditLimitExceeded), errors.Is(err, sales.ErrCustomerInactive), errors.Is(err, sales.ErrProductInactive), errors.Is(err, customers.ErrCreditRiskHold), errors.Is(err, customers.ErrCreditOverdue),
		errors.Is(err, sales.ErrOfflineCredit), errors.Is(err, sales.ErrOfflinePaymentMethod), errors.Is(err, sales.ErrOfflineTaxUnsupported), errors.Is(err, sales.ErrUnsupportedPayment), errors.Is(err, devices.ErrNotActive), errors.Is(err, devices.ErrOfflineDisabled), errors.Is(err, devices.ErrMobileCreditUnsupported), errors.Is(err, devices.ErrAllocationExceeded), errors.Is(err, devices.ErrOfflineLimit), errors.Is(err, devices.ErrOfflineLeaseExpired), errors.Is(err, devices.ErrStaleMasterData), errors.Is(err, devices.ErrInvalidTimeZone), errors.Is(err, devices.ErrInvalidStatusTransition):
		status, code = http.StatusUnprocessableEntity, "business_rule_violation"
	case errors.Is(err, sales.ErrInvalidCommand), errors.Is(err, sales.ErrInvalidLine), errors.Is(err, sales.ErrDuplicateProductLine), errors.Is(err, sales.ErrInvalidSaleKind), errors.Is(err, sales.ErrPaymentMethodRequired), errors.Is(err, operations.ErrInvalidCommand), errors.Is(err, banking.ErrInvalidCommand), errors.Is(err, financialops.ErrInvalidCommand):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, reporting.ErrInvalidQuery):
		status, code = http.StatusBadRequest, "invalid_report_query"
	case errors.Is(err, advancedfinance.ErrInvalidCommand):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, advancedfinance.ErrInvalidTransition), errors.Is(err, advancedfinance.ErrSeparationOfDuties), errors.Is(err, advancedfinance.ErrAssetFullyDepreciated):
		status, code = http.StatusConflict, "advanced_finance_conflict"
	case errors.Is(err, treasury.ErrInvalidCommand):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, treasury.ErrInvalidTransition), errors.Is(err, treasury.ErrSeparationOfDuties), errors.Is(err, treasury.ErrLimitExceeded):
		status, code = http.StatusConflict, "treasury_conflict"
	case errors.Is(err, groupfinance.ErrInvalidCommand):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, groupfinance.ErrInvalidTransition), errors.Is(err, groupfinance.ErrSeparationOfDuties), errors.Is(err, groupfinance.ErrCounterpartyApproval):
		status, code = http.StatusConflict, "group_finance_conflict"
	}
	if status == http.StatusInternalServerError {
		h.logger.ErrorContext(request.Context(), "request failed", "method", request.Method, "path", request.URL.Path, "error", err)
		writeProblem(writer, status, code, "the request could not be completed")
		return
	}
	writeProblem(writer, status, code, err.Error())
}

func writeProblem(writer http.ResponseWriter, status int, code, detail string) {
	writeJSONStatus(writer, status, map[string]any{
		"type": "about:blank", "title": http.StatusText(status), "status": status,
		"code": code, "detail": detail, "correlation_id": correlationID(writer),
	}, "application/problem+json")
}

func correlationID(writer http.ResponseWriter) string { return writer.Header().Get("X-Correlation-ID") }

func correlationIDs(generator identity.Generator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		value := strings.TrimSpace(request.Header.Get("X-Correlation-ID"))
		if !correlationIDPattern.MatchString(value) {
			generated, err := generator.New()
			if err != nil {
				http.Error(writer, "unable to establish request correlation", http.StatusInternalServerError)
				return
			}
			value = generated
		}
		writer.Header().Set("X-Correlation-ID", strings.ToLower(value))
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writeJSONStatus(writer, status, value, "application/json")
}
func writeJSONStatus(writer http.ResponseWriter, status int, value any, contentType string) {
	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(writer, request)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (writer *statusRecorder) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusRecorder) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func (h *Handler) observe(next http.Handler) http.Handler {
	logger := h.logger
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer}
		next.ServeHTTP(recorder, request)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.InfoContext(request.Context(), "http request completed",
			"method", request.Method,
			"route", request.Pattern,
			"path", request.URL.Path,
			"status", status,
			"latency_ms", time.Since(startedAt).Milliseconds(),
			"correlation_id", recorder.Header().Get("X-Correlation-ID"),
		)
	})
}
