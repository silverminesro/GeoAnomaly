import 'package:flutter/material.dart';

class ErrorUtils {
  static String formatError(dynamic error) {
    String errorMessage = error.toString();
    
    // Remove "Exception: " prefix if present
    if (errorMessage.startsWith('Exception: ')) {
      errorMessage = errorMessage.substring('Exception: '.length);
    }
    
    // Handle common HTTP errors
    if (errorMessage.contains('Failed to login')) {
      return 'Invalid username or password';
    }
    
    if (errorMessage.contains('Failed to get user profile')) {
      return 'Unable to load user profile. Please try again.';
    }
    
    if (errorMessage.contains('Failed to get inventory')) {
      return 'Unable to load inventory. Please check your connection.';
    }
    
    if (errorMessage.contains('SocketException')) {
      return 'No internet connection. Please check your network.';
    }
    
    if (errorMessage.contains('TimeoutException')) {
      return 'Request timed out. Please try again.';
    }
    
    if (errorMessage.contains('401')) {
      return 'Session expired. Please log in again.';
    }
    
    if (errorMessage.contains('403')) {
      return 'Access denied. Please check your permissions.';
    }
    
    if (errorMessage.contains('404')) {
      return 'Resource not found.';
    }
    
    if (errorMessage.contains('500')) {
      return 'Server error. Please try again later.';
    }
    
    return errorMessage;
  }
  
  static void showErrorSnackBar(BuildContext context, dynamic error) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            const Icon(Icons.error, color: Colors.white),
            const SizedBox(width: 8),
            Expanded(
              child: Text(formatError(error)),
            ),
          ],
        ),
        backgroundColor: Theme.of(context).colorScheme.error,
        duration: const Duration(seconds: 4),
        action: SnackBarAction(
          label: 'Dismiss',
          textColor: Colors.white,
          onPressed: () {
            ScaffoldMessenger.of(context).hideCurrentSnackBar();
          },
        ),
      ),
    );
  }
  
  static void showSuccessSnackBar(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            const Icon(Icons.check_circle, color: Colors.white),
            const SizedBox(width: 8),
            Expanded(
              child: Text(message),
            ),
          ],
        ),
        backgroundColor: Colors.green,
        duration: const Duration(seconds: 3),
      ),
    );
  }
}