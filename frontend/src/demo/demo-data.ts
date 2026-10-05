import type { Address } from '../api/types';

/** A seeded demo user, mirroring cmd/seed. */
export interface DemoSeedUser {
  username: string;
  name: string;
  phone: string;
  address: Address;
}

/** A person known to a mock identity vendor, already mapped to the domain shape. */
export interface DemoVendorRecord {
  name: string;
  phone: string;
  address: Address;
}

const addr = (
  street_address = '',
  locality = '',
  region = '',
  postal_code = '',
  country = '',
): Address => ({ street_address, locality, region, postal_code, country });

/** Same users as cmd/seed so the hosted demo behaves like the local stack. */
export const DEMO_SEED_USERS: readonly DemoSeedUser[] = [
  {
    username: 'admin',
    name: 'Rolodex Admin',
    phone: '+14165550100',
    address: addr('', 'Toronto', 'ON', '', 'CA'),
  },
  {
    username: 'ada',
    name: 'Ada Lovelace',
    phone: '+14165550101',
    address: addr('100 King St W', 'Toronto', 'ON', 'M5X 1A9', 'CA'),
  },
  {
    username: 'grace',
    name: 'Grace Hopper',
    phone: '+12125550102',
    address: addr('', 'New York', 'NY', '', 'US'),
  },
  { username: 'alan', name: 'Alan Turing', phone: '+442079460103', address: addr() },
  {
    username: 'katherine',
    name: 'Katherine Johnson',
    phone: '+17575550104',
    address: addr('1 NASA Dr', 'Hampton', 'VA', '23681', 'US'),
  },
];

/** Mirrors internal/mockvendor ABCRecords after the ABC adapter's normalisation. */
export const DEMO_ABC_RECORDS: readonly DemoVendorRecord[] = [
  {
    name: 'Ada Lovelace',
    phone: '+14165550101',
    address: addr('100 King St W', 'Toronto', 'ON', 'M5X 1A9', 'CA'),
  },
  {
    name: 'Grace Hopper',
    phone: '+12125550102',
    address: addr('350 Fifth Ave', 'New York', 'NY', '10118', 'US'),
  },
  {
    name: 'Katherine Johnson',
    phone: '+17575550104',
    address: addr('12 Old Mill Rd', 'Hampton', 'VA', '23666', 'US'),
  },
];

/** Mirrors internal/mockvendor XYCRecords after the XYC adapter's normalisation. */
export const DEMO_XYC_RECORDS: readonly DemoVendorRecord[] = [
  {
    name: 'Ada Lovelace',
    phone: '+14165550101',
    address: addr('100 King St W', 'Toronto', 'ON', 'M5X1A9', 'CA'),
  },
  {
    name: 'Alan Turing',
    phone: '+442079460103',
    address: addr('1 Bletchley Park', 'Milton Keynes', 'Buckinghamshire', 'MK3 6EB', 'GB'),
  },
  {
    name: 'Katherine Johnson',
    phone: '+17575550104',
    address: addr('1 NASA Dr', 'Hampton', 'VA', '23681', 'US'),
  },
];
