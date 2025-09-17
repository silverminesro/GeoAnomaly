import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'features/inventory/screens/inventory_screen.dart';
import 'core/services/api_service.dart';
import 'core/services/auth_service.dart';
import 'core/screens/login_screen.dart';
import 'features/inventory/services/inventory_service.dart';

void main() {
  runApp(const GeoAnomalyApp());
}

class GeoAnomalyApp extends StatelessWidget {
  const GeoAnomalyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        Provider<AuthService>(
          create: (_) => AuthService(),
        ),
        Provider<ApiService>(
          create: (_) => ApiService(),
        ),
        ProxyProvider<ApiService, InventoryService>(
          update: (_, apiService, __) => InventoryService(apiService),
        ),
      ],
      child: MaterialApp(
        title: 'GeoAnomaly',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: Colors.deepPurple,
            brightness: Brightness.dark,
          ),
          scaffoldBackgroundColor: const Color(0xFF121212),
          appBarTheme: const AppBarTheme(
            backgroundColor: Color(0xFF1F1F1F),
            elevation: 4,
          ),
          cardTheme: CardTheme(
            elevation: 4,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
          inputDecorationTheme: InputDecorationTheme(
            filled: true,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
            ),
          ),
        ),
        initialRoute: '/',
        routes: {
          '/': (context) => const AuthWrapper(),
          '/login': (context) => const LoginScreen(),
          '/inventory': (context) => const InventoryScreen(),
        },
        debugShowCheckedModeBanner: false,
      ),
    );
  }
}

class AuthWrapper extends StatelessWidget {
  const AuthWrapper({super.key});

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<bool>(
      future: context.read<AuthService>().isLoggedIn(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Scaffold(
            body: Center(
              child: CircularProgressIndicator(),
            ),
          );
        }

        if (snapshot.data == true) {
          // Auto-set token if logged in
          _setTokenFromStorage(context);
          return const InventoryScreen();
        } else {
          return const LoginScreen();
        }
      },
    );
  }

  Future<void> _setTokenFromStorage(BuildContext context) async {
    final authService = context.read<AuthService>();
    final apiService = context.read<ApiService>();
    final token = await authService.getToken();
    if (token != null) {
      apiService.setToken(token);
    }
  }
}