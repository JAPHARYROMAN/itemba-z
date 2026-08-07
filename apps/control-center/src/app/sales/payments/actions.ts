"use server";

import { createServerRepository } from "@/live-api/server-repository";
import { publicProblem } from "@/live-api/errors";
import type { CustomerAccountDetail, PublicProblem } from "@/live-api/types";

export type CustomerPaymentAccountResult =
  | { ok: true; account: CustomerAccountDetail }
  | { ok: false; problem: PublicProblem };

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function invalidCustomerProblem(detail: string): CustomerPaymentAccountResult {
  return {
    ok: false,
    problem: {
      type: "urn:itemba-z:control-center:customer-payment",
      title: "Customer account unavailable",
      status: 400,
      code: "customer_payment_customer_invalid",
      detail,
    },
  };
}

export async function loadCustomerPaymentAccount(customerId: string): Promise<CustomerPaymentAccountResult> {
  const normalizedCustomerId = customerId.trim();
  if (!UUID_PATTERN.test(normalizedCustomerId)) {
    return invalidCustomerProblem("Choose a valid customer before loading open invoices.");
  }

  try {
    const repository = await createServerRepository();
    const account = await repository.getCustomerAccount(normalizedCustomerId);
    if (account.customer.id !== normalizedCustomerId || account.customer.is_general_customer) {
      return invalidCustomerProblem("Payments can only be allocated to a named customer account.");
    }
    return { ok: true, account };
  } catch (error) {
    return { ok: false, problem: publicProblem(error) };
  }
}
