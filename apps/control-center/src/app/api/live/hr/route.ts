import type { AttendanceCommand, CreateEmployeeCommand, CreateShiftAssignmentCommand, CreateShiftTemplateCommand, GeneratePayrollArtifactCommand, LeaveCommand, LeaveTypeCommand, LoanCommand, PayrollCommand, PeopleTransitionCommand, RegisterEmployeeDocumentCommand, WorkforceStatus } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
type Command =
  | { action: "create_employee"; command: CreateEmployeeCommand }
  | { action: "record_attendance"; command: AttendanceCommand }
  | { action: "create_leave_type"; command: LeaveTypeCommand }
  | { action: "create_leave"; command: LeaveCommand }
  | { action: "transition_leave"; id: string; command: PeopleTransitionCommand }
  | { action: "create_loan"; command: LoanCommand }
  | { action: "transition_loan"; id: string; command: PeopleTransitionCommand }
  | { action: "create_payroll"; command: PayrollCommand }
  | { action: "transition_payroll"; id: string; command: PeopleTransitionCommand }
  | { action: "create_shift_template"; command: CreateShiftTemplateCommand }
  | { action: "transition_shift_template"; id: string; status: WorkforceStatus; reason: string }
  | { action: "create_shift_assignment"; command: CreateShiftAssignmentCommand }
  | { action: "transition_shift_assignment"; id: string; status: WorkforceStatus; reason: string }
  | { action: "register_employee_document"; command: RegisterEmployeeDocumentCommand }
  | { action: "generate_payroll_artifact"; id: string; command: GeneratePayrollArtifactCommand };
export async function POST(request: Request) {
  try {
    assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<Command>(request); const repository = await createServerRepository();
    switch (body.action) {
      case "create_employee": return liveResponse(await repository.createEmployee(body.command, key), 201);
      case "record_attendance": return liveResponse(await repository.recordAttendance(body.command, key), 201);
      case "create_leave_type": return liveResponse(await repository.createLeaveType(body.command, key), 201);
      case "create_leave": return liveResponse(await repository.createLeave(body.command, key), 201);
      case "transition_leave": return liveResponse(await repository.transitionLeave(body.id, body.command, key));
      case "create_loan": return liveResponse(await repository.createEmployeeLoan(body.command, key), 201);
      case "transition_loan": return liveResponse(await repository.transitionEmployeeLoan(body.id, body.command, key));
      case "create_payroll": return liveResponse(await repository.createPayroll(body.command, key), 201);
      case "transition_payroll": return liveResponse(await repository.transitionPayroll(body.id, body.command, key));
      case "create_shift_template": return liveResponse(await repository.createShiftTemplate(body.command, key), 201);
      case "transition_shift_template": return liveResponse(await repository.transitionShiftTemplate(body.id, body.status, body.reason, key));
      case "create_shift_assignment": return liveResponse(await repository.createShiftAssignment(body.command, key), 201);
      case "transition_shift_assignment": return liveResponse(await repository.transitionShiftAssignment(body.id, body.status, body.reason, key));
      case "register_employee_document": return liveResponse(await repository.registerEmployeeDocument(body.command, key), 201);
      case "generate_payroll_artifact": return liveResponse(await repository.generatePayrollArtifact(body.id, body.command, key), 201);
    }
  } catch (error) { return problemResponse(error); }
}
