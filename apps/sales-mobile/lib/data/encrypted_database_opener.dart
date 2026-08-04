import 'package:sqflite_sqlcipher/sqlite_api.dart';
import 'package:sqflite_sqlcipher/sqflite.dart' as sqlcipher;

abstract interface class EncryptedDatabaseOpener {
  Future<Database> open({
    required String path,
    required String password,
    required int version,
    required OnDatabaseConfigureFn onConfigure,
    required OnDatabaseCreateFn onCreate,
    required OnDatabaseVersionChangeFn onUpgrade,
  });
}

class SqlCipherDatabaseOpener implements EncryptedDatabaseOpener {
  const SqlCipherDatabaseOpener();

  @override
  Future<Database> open({
    required String path,
    required String password,
    required int version,
    required OnDatabaseConfigureFn onConfigure,
    required OnDatabaseCreateFn onCreate,
    required OnDatabaseVersionChangeFn onUpgrade,
  }) {
    return sqlcipher.openDatabase(
      path,
      password: password,
      version: version,
      onConfigure: onConfigure,
      onCreate: onCreate,
      onUpgrade: onUpgrade,
      singleInstance: true,
    );
  }
}
