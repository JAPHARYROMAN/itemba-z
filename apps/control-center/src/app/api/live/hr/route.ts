import type { AttendanceCommand, CreateEmployeeCommand, LeaveCommand, LeaveTypeCommand, LoanCommand, PayrollCommand, PeopleTransitionCommand } from "@/live-api/types";
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
  | { action: "transition_payroll"; id: string; command: PeopleTransitionCommand };
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
    }
  } catch (error) { return problemResponse(error); }
}
